# Round 1, seat logic: review of the plan go-s1-stack-vault

Task identifier: review-go-s1-stack-vault-r1-logic. Nonce: 6ac2d9. Model and effort: claude-sonnet-5-5, high.

## Verdict

REVISE

## Top finding

The first-start script keeps its state in six loose files and never says which combination means "done", "resume", or "stop", so a run after `docker compose down -v` or after a half-finished first run lands in cells the plan leaves empty (finding logic-r1 2).

## What I ran

Docker worked in this review session, although `scouting.md` says it was denied during planning. I ran throwaway containers in a scratch folder outside the repository and removed them: no container, volume or network is left, and the scratch folder holds no key material. The results below are marked "verified".

- Both pinned tags exist and pull: `openbao/openbao:2.7.3` and `postgres:16.16-alpine`.
- The PostgreSQL image runs as user and group 70. The OpenBao image runs as user 100 by default, and its default command is `server -dev -dev-no-store-token`.
- With the plan's storage and static seal blocks, a raw 32-byte key file, and a role that owns its database, OpenBao stored its data in `public.openbao_kv_store`. After a container replacement it came back initialized and unsealed with no unseal call. Both upstream facts hold.
- With the key file missing, the container exits with code 1 and a clear message. With a different random 32-byte key on an initialized vault, the container keeps running, stays sealed, and logs a decryption failure. Restoring the original key reopened the vault.
- `PUT /v1/sys/init` with one recovery share returns `recovery_keys_base64` and `root_token`, as the plan assumes. A second init is refused with HTTP 400.

## Findings

### logic-r1 1. MAJOR. Tied to D10.

The OpenBao role cannot create its own table, so the first OpenBao start fails.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:125` grants only `CONNECT` and forbids database creation. It never makes `lgtm_bao` the owner of its database and never grants `CREATE` on schema `public`. `.plans/plans/go-s1-stack-vault/plan.md:137` relies on OpenBao to create `openbao_kv_store`. The planned storage block does not set `skip_create_table`, so OpenBao creates the table itself.
- Failure scenario (verified): a database created by the administrator, `CONNECT` granted to `lgtm_bao`, no ownership. OpenBao exits with `failed to create tables: ... permission denied for schema public (SQLSTATE 42501)`. With `ALTER DATABASE lgtm_bao OWNER TO lgtm_bao` it starts. PostgreSQL 15 and later no longer let ordinary roles create objects in `public`. The disposable proof in `scouting.md:96-111` and `plan.md:73` uses the PostgreSQL superuser for OpenBao, so it cannot find this.
- Fix: state in item 2 that `lgtm_app` owns database `lgtm_app` and `lgtm_bao` owns database `lgtm_bao`, with `CREATE DATABASE ... OWNER`. Owning a database does not grant `CREATEDB`, so the plan's "cannot create other databases" stays true. Add to `postgres_roles_are_isolated` an assertion that OpenBao started as `lgtm_bao` creates `openbao_kv_store`. The item text should also reject `skip_create_table` and a `CREATE` grant to `PUBLIC` as alternative fixes.

### logic-r1 2. MAJOR. Tied to D10 and C13.

The first-start script has no defined state machine, and the plan leaves cells of its truth table empty.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:137` says "private bootstrap journal" and "published credential". `plan.md:202` says "setup record" and "trusted bootstrap material". `plan.md:204` says "the recorded step" and "durable revocation marker". The files that exist are `admin/init-response.json`, `admin/recovery.key`, `admin/root.token`, `admin/server-secret-id.pending`, `admin/server-openbao.pending.json`, `admin/root.revoked` and `server/openbao.json`. No sentence says which subset means which step. `plan.md:204` defines "completed" only as a "valid completed install" and says nothing about what the script checks.
- Failure scenario, empty cell A: the operator runs `docker compose -f deploy/compose.yaml down -v`, the usual way to reset. `plan.md:223` says the state directory persists. PostgreSQL starts fresh. `stack-prepare database` reuses the old password files. `seal` keeps the old key, because the key exists and the plan only guards creating a key. The vault is uninitialized with no rows, but `server/openbao.json` exists. The plan only defines init for "no setup record or credential file", and only defines the completed branch for a valid file. If the completed check is by file, the script prints `vault already configured` over an empty vault, and the sprint 2 server then fails every login forever. If the key was also deleted, `seal` refuses with the message `seal key missing for existing vault`, which is false (the vault is empty) and has no way out.
- Failure scenario, empty cell B: the credential file was published but `admin/*.pending` files remain after a crash following the rename. Is that "complete" or "resume"? The plan does not say.
- Failure scenario, empty cell C: the operator deletes `server/openbao.json` to rotate it. The store is initialized, `root.revoked` exists, and `recovery.key` exists. `plan.md:204` calls this "incomplete state" and fails closed, but the plan also says at line 206 that an administrator can replace the identifier using the recovery share. The script gives no path to do that.
- Fix: replace the loose files with one `admin/bootstrap.state` file written by atomic rename, holding a single step name from a closed list (for example `initialized`, `provisioned`, `revoked`, `published`). Add to the plan a truth table with rows `(store initialized: yes or no) x (state step) x (credential file: absent, valid, invalid)` and one outcome per row. Require that the completed branch also checks `GET /v1/sys/init` reports initialized; this is a status read, not a provisioning call or login. For an uninitialized store with leftover local state, exit with a distinct diagnostic and name the repair in `deploy/README.md`. Use one name for the record everywhere in the plan.

### logic-r1 3. MAJOR. Tied to D10.

The window between a committed `PUT /v1/sys/init` and a durable `init-response.json` ends in a refusal with no repair.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:202` stops with `vault is initialized but bootstrap state is incomplete` and says "Do not attempt another initialization or discard the database". The README contents at `plan.md:225` list only the missing-key diagnostic.
- Failure scenario: `up.sh` is killed, or the process dies, after OpenBao committed the init but before the response was written. This is the first start, so the vault holds nothing but its own initialization. The next run finds an initialized store and no material, prints the diagnostic, and every later run prints it again. The only exit is to wipe the database and the state by hand, which the plan forbids and does not document. The failure denies and never repairs.
- Fix: say that at this exact point (store initialized, no step file, no recovery key, no credential, and no keys under the `lgtm` mount) the vault is provably empty, and add a guarded `stack-prepare reset-vault` command. It stops OpenBao, deletes only the rows of `openbao_kv_store`, and removes `bao/` and `admin/`. It refuses unless that proof holds. Name the command in the diagnostic and in the README. A test `bootstrap_recovers_from_lost_init_response` kills the job between init and the first file write and expects a clean second start.

### logic-r1 4. MAJOR. Tied to D4 and R10.

The PostgreSQL health check does not authenticate, so the plan's claim "authenticated query" is false for the form it prescribes.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:125` runs `SELECT 1` "through PostgreSQL's loopback network listener". `plan.md:131` and the acceptance list at `plan.md:104` repeat that it is authenticated.
- Failure scenario (verified): in the pinned image, even with `POSTGRES_HOST_AUTH_METHOD=scram-sha-256`, `pg_hba.conf` keeps the IPv4 loopback `trust` line and the `local` trust lines ahead of the appended `scram-sha-256` line. A loopback connection with a wrong password returned `1`. The same wrong password from another container was refused. If the generated password file for `lgtm_bao` differs from the role's real password (a stale file after a partial first start, for example), PostgreSQL reports healthy, OpenBao starts, and OpenBao then waits forever or fails. The mutation "replace the check with a Unix-socket probe" at `plan.md:133` is still caught because the socket exists earlier, but a check that proves the password files match the roles is never exercised.
- Fix: give the image `POSTGRES_INITDB_ARGS=--auth=scram-sha-256` in addition to the host setting so that loopback and local are also password-checked, or probe through the container's non-loopback address. Add to `postgres_waits_for_both_databases` an assertion that a connection with a wrong password through the health-check path fails.

### logic-r1 5. MAJOR. Tied to C13 and the plan's step 1.

The disposable proof, which gates everything, tests a different configuration from the one the stack will use, and `scouting.md` records no output.

- Evidence: `.plans/plans/go-s1-stack-vault/scouting.md:96-111` connects OpenBao as the PostgreSQL superuser, initializes with the command line, starts with `--entrypoint bao`, and never tries a wrong key. `plan.md:73` and `plan.md:81` copy that shape. The production service uses role `lgtm_bao`, the HTTP init call, the image entrypoint, and a key owned by user 10002. `scouting.md:51` says the facts "remain documented claims", and no command output is recorded for question (a) of the task.
- Failure scenario: the proof passes with the superuser and the production start then fails on the table creation of finding 1. A wrong key (the realistic failure after a restore mistake) behaves differently from a missing key: the container keeps running and sealed instead of exiting (verified). `plan.md:79` tests only the missing-key behavior, and `plan.md:141` says a wrong key "must still fail the health check" without a test.
- Fix: change the proof so that OpenBao connects as an owner-only, non-superuser role, initializes through `PUT /v1/sys/init` with `recovery_shares=1`, and starts through the image entrypoint with the same `command` as production. Add a fourth assertion `wrong_seal_key_stays_sealed` (running or exited, but `sealed=true` or no health, and the row count unchanged). Record the observed image digests and the output lines in `scouting.md` before the plan leaves review. The planner can reuse the transcript summarized under "What I ran".

### logic-r1 6. MINOR. Tied to D10.

Some guard branches of `seal` and the last launcher step have no test, and a failure there is a bare timeout.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:137` blocks key creation on any row, on a bootstrap journal, or on a published credential. `plan.md:146` tests only the row case. `plan.md:223` ends with `up -d --wait --wait-timeout 60`.
- Failure scenario: an implementation that checks only rows passes all three listed tests, so the journal and credential guards, which protect the empty-database cases of finding 2, are untested. A wrong key, or a changed `lgtm_bao` password, leaves OpenBao running but unhealthy and the launcher fails after 60 seconds with Compose's generic timeout and no diagnostic. With `max_connect_retries=0` the plan relies on unlimited retries (the upstream text says zero means unlimited), so a wrong database password hangs instead of exiting.
- Fix: add tests `seal_refuses_with_journal_and_empty_database` and `seal_refuses_with_credential_and_empty_database`. In `up.sh`, on a failed final wait, print the last log lines of `openbao` classified as "seal key does not open the vault" or "database refused the login" and exit nonzero.

### logic-r1 7. MINOR. Tied to D10.

The OpenBao service definition leaves out settings that decide whether it runs a real vault at all.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:139-141`.
- Failure scenario (verified): the image's default command is `server -dev -dev-no-store-token`. A compose service that omits `command` runs an in-memory development vault, which is the plan's own High risk at `plan.md:87`. The plan does not name the `command`, the config mount path (the entrypoint reads `/openbao/config`), or the wrapper that builds `BAO_PG_CONNECTION_URL` from the password file (the image entrypoint cannot do that). It does not say that the URL needs `sslmode=disable`; the upstream page warns that the backend tries SSL. `disable_mlock = true` at `plan.md:139` produced `unknown or unsupported field disable_mlock` in 2.7.3 and has no effect.
- Fix: list `command: server`, the config mount, the wrapper script name and its owner (item 3), and `sslmode=disable` in the plan. Remove `disable_mlock`. Add `openbao_runs_with_postgres_storage` to the seal test: `storage_type=postgresql` in `bao status`.

### logic-r1 8. MINOR. Tied to D10.

Images are pinned by tag only, and the helper build is not reproducible.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:115` and `plan.md:141` pin `postgres:16.16-alpine` and `openbao/openbao:2.7.3`. `plan.md:185` installs `curl` and `jq` with no version. The digests are only to be recorded in `scouting.md` (`plan.md:44`).
- Failure scenario: a retagged image or a new `jq` changes behavior silently between two installations, and the recorded digests do not bind the compose file.
- Fix: write the digests into `deploy/compose.yaml` as `image: name:tag@sha256:...` after the proof, and pin the two package versions in the Dockerfile.

### logic-r1 9. MINOR. Tied to plan.

Two work items that run in sequence share files, and one item's test fixture depends on a file that another item creates.

- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:115`, `137` and `200` show items 2, 3 and 5 all editing `deploy/compose.yaml`. Items 2 and 3 both edit `deploy/stack-prepare.sh`. `plan.md:185` says item 4 can run "before item 2" while `deploy/test/vault-testlib.sh` is created in item 2 (`plan.md:115`).
- Failure scenario: item 4's tests need a fixture helper that does not exist yet, or two builders create it twice.
- Fix: say that item 4 carries its own copy of the fixture functions, or move `vault-testlib.sh` to item 1's deliverable, and mark items 2, 3 and 5 in the sizing table as strictly sequential on `deploy/compose.yaml`.

## Answers to the attack questions

Lens questions:

1. Sequences, counters, expiries: the plan introduces few. The AppRole token lifetime (15 minutes) and the 60-second waits run on one host clock. The only ordered writes are the init steps, covered in finding 2. Nothing found on ties or wraps; checked `plan.md:187`, `plan.md:204-206`.
2. Failures: database down, crash mid-write and duplicate runs are addressed for the credential and the key; the gaps are findings 2, 3 and 6. A running vault whose database goes down may still answer `bao status` as unsealed, so the final health pass says little about storage; this is a post-build check.
3. Truth table of rules on the same state (key, vault database, journal, credential, PostgreSQL volume): the empty cells are in finding 2.
4. Zero values: `secret_id_num_uses=0` and `secret_id_ttl=0` mean unlimited and are intended. `max_connect_retries=0` means unlimited retries (finding 6). An empty `LGTM_STACK_STATE_DIR` set as an empty string differs from an absent variable at `plan.md:117`; the launcher resolves the default, but a Compose run without the launcher would bind an empty path. Add a Compose `:?` guard.
5. Two actors on one file or row: the launcher lock covers `up.sh` only, at `plan.md:221`. A direct `docker compose up` by the operator, or the `restart: unless-stopped` daemons, take no lock, but they do not write the setup files, so nothing found beyond finding 9.
6. Risky items and their mutations: item 2's mutations are caught, but the health check test does not catch a wrong password (finding 4). Item 3's `0644` mutation is caught. A wrong change the tests would not catch: `seal` that creates a key when there are no rows and a journal exists (finding 6).
7. Cheapest wrong implementation that passes all listed tests: a `seal` that checks only the row count (finding 6); a health check on loopback with no password (finding 4); an init job that never verifies that the store is initialized in the completed branch (finding 2).
8. A shape that makes the wrong state impossible: one step file written by rename in place of six loose files (finding 2); database ownership by the role in place of grant lists (finding 1).

Task questions:

a. The two facts were not run by the planner, and `scouting.md` records no image digest, command or output. The plan defers to work item 1 and gates later work on it. I ran both and they hold (see "What I ran"), with two differences from the plan: finding 1 and finding 5. The plan follows from the facts it assumed; it does not follow from the facts as observed with a restricted role.
b. A second run after a complete first run is specified: `vault already configured`, no credential rewrite, no provisioning call, no login (`plan.md:204`). Whether "complete" is judged by file or by the store is not stated (finding 2). A run after a half-finished first run resumes at "the recorded step", which is undefined (finding 2), and the lost-response window has no repair (finding 3). The credential file is `deploy/.state/server/openbao.json` by default and `server/openbao.json` under `LGTM_STACK_STATE_DIR`, a JSON object with exactly `role_id` and `secret_id` and a trailing newline, owner and group 10001, mode `0400`, mounted at `/run/lgtm/openbao.json` (`plan.md:206`). It is replaced by an administrator using the recovery share, with no tool provided and a container recreate needed; the plan states this at `plan.md:206` but gives no command (finding 2, cell C).
c. Pinned: by tag, not digest (finding 8). Ports: no host port for either daemon, and the networks are `data` and `vault` (`plan.md:115`, `141`); good, and the first publish arrives in sprint 2. Health checks: PostgreSQL has one that does not authenticate (finding 4), and OpenBao uses `bao status`, which I verified exits 2 while uninitialized or sealed and 0 once unsealed. Start order: `depends_on` healthy PostgreSQL, then seal, then OpenBao, then init, then a final wait (`plan.md:223`); consistent. PostgreSQL holds two databases and two roles apart (`lgtm_app`, `lgtm_bao`), but without ownership the second cannot work (finding 1).

## Post-build checks

1. After finding 4 is fixed: connect to `postgres` through the health-check path with a wrong password and expect a refusal.
2. Kill the init job at each file write of the first start, one run per point, then run `up.sh` again. Expected: the run reaches `postgres and openbao are healthy`, or prints one documented diagnostic with a repair that works.
3. Run `docker compose down -v`, then `up.sh`. Expected: a distinct diagnostic, never `vault already configured` over an empty vault.
4. Restart the Docker daemon while the key file is missing. Expected: the container restart loop is visible in the logs with the missing-key message, and the launcher still refuses.
5. Bind the single credential file from a root-owned `0700` directory into a container running as user 10001. Expected: the file reads, and the server container is recreated after an atomic replace of the file.
6. First-time PostgreSQL initialization on a slow disk. Expected: `--wait-timeout 60` is enough, or the timeout is raised.
7. Stop PostgreSQL while OpenBao runs. Expected: `bao status` health is documented as not covering storage, or the health check includes a read.

## Grade

5 out of 10. 0 blockers, 5 majors, 4 minors.
