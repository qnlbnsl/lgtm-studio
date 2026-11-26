---
topic: go-s1-stack-vault
status: done
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# PostgreSQL and OpenBao start together and reopen the vault after restart

## Overview

One command starts the sprint 1 stack. OpenBao stores data in its own PostgreSQL database and unseals with the same local key after restart.

The first build item repeats the storage and unseal proof with production identities. The sprint 2 server gets a credential for its signing-key path.

## Key decisions

On 2026-11-26, the orchestrator kept six work items. First start saves the administrator response and publishes the server credential after login. Repeat launches verify the credential. (architect-r3 1, architect-r3 3, logic-r3 1, logic-r3 4)

Startup locks the state directory and checks the passwords and seal. Only a known unfinished disposable installation can be rebuilt as part of wave 1. Restoring PostgreSQL or the vault state directory to an earlier state is outside wave 1. (architect-r3 2, logic-r3 2, logic-r3 3)

[Full decision log](decisions.md)

## Workstreams

### Prove storage and give the Server team a test database

The proof checks that writing to the vault adds PostgreSQL records. It replaces both services and reads back the same value. The separate test wrapper gives Server a disposable database for Go tests and the change-feed mutation.

### Start the two services with their original credentials

Each daemon uses its own database login. The launcher checks PostgreSQL before preparing the seal. Existing vault data or initialization files prevent automatic replacement of a missing key.

### Give the server a restricted vault credential and restart the stack

First start creates the restricted role and validates its credential before publication. After automatic unseal, the same credential reads the original value.

## Rollout order

1. Run the disposable production-shape proof. Record the pinned images, the row growth and the restart results.
2. Prepare the database and vault. Verify role isolation, authenticated readiness and the seal controls.
3. Launch twice, restart the stack and read the same fixture signing key. Run the real-database check once the Server store merges. Hand the runtime contracts to sprint 2.

---
<!-- ⛔ implementation details and gate evidence below -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: Prove storage and give the Server team a test database**

The disposable proof establishes the exact image behavior. The test database supports the build without publishing a production port.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1a | The vault uses its owned PostgreSQL database and reads the original value after both services restart. | `postgres_storage_round_trip` and `static_seal_restart`. |
| 1b | Missing and wrong seal keys leave the vault closed. The original key still opens it. The fixture is removed. | `missing_seal_key_blocks_unseal`, `wrong_seal_key_stays_sealed` and `cleanup=true`. |
| 1c | The host wrapper creates the named maintenance database and passes the Server mutation result through the existing driver. | `test_database_is_disposable`, `mutation_hook_passes_through_server_status` and PB5 after the Server merge. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Run the disposable production-shape proof. | `deploy/openbao.hcl`, planned `storage "postgresql"` and `seal "static"`. [Proof commands and anchors](scouting.md#production-shape-proof-for-work-item-1). |
| 8 | Provide the disposable test database and change-feed hook. | `deploy/test/with-postgres.sh` and `deploy/test/mutate-feed-lock.sh`, following `deploy/test/checks.sh`, `mutation_hook` and `check_c12`. |

**Work item 1. Prove the pinned images**

Keep `proof.sh`, `proof.test.sh` and `bao.hcl` in a disposable directory. Use the [production-shape proof](scouting.md#production-shape-proof-for-work-item-1). Stack sends the sanitized exact-commit result to the planner or orchestrator, who updates scouting. Build uses the pinned images observed by the earlier reviewers. (architect-r1 1, logic-r1 5)

- `postgres_storage_round_trip` checks that the unprivileged `lgtm_bao` owner creates `public.openbao_kv_store`, and that writing a sample increases the row count. Replacing both services preserves the value.
- `static_seal_restart` requires `initialized=true`, `sealed=false` and `storage_type=postgresql` within 60 seconds, without an unseal command.
- `missing_seal_key_blocks_unseal` and `wrong_seal_key_stays_sealed` each preserve the rows. Use an absent file and a different 32-byte file, respectively. Restoring the original key recovers the sample.

Freeze the assertions before running the proposed fixture. This item is risky. Switch to in-memory storage or change the restart key and require the corresponding test to fail. Acceptance is four named passing assertions, the image digests and `cleanup=true`.

**Work item 8. Provide the real test database and rollout hook**

Create executable `deploy/test/with-postgres.sh` and `deploy/test/mutate-feed-lock.sh`. Create `deploy/test/vault-fixtures.sh` for these two checks and update `deploy/test/README.md`. Stack owns the files.

`deploy/test/with-postgres.sh -- <command>` runs a disposable instance of the pinned PostgreSQL image with temporary storage and a dynamically assigned port on loopback. Create the maintenance database `lgtm_store_test` owned by the test login. Give that login the `CREATEDB` privilege. Stack sets `LGTM_TEST_POSTGRES` to that database for the child command only. Server's `storetest.NewDatabase`, the Go suite and `check_c12` read it. Supply the instance password through a private mounted file. Clean up the instance and private files on normal exit and handled interruption. No password or connection string appears in arguments, Docker configuration values or output. (architect-r3 5, architect-r2 3, threat-r2 5)

Stack's hook executes Server's `src/server/internal/store/check-feed-lock.sh` with the inherited setting and preserves its exit status. A missing or nonexecutable Server script fails with a fixed diagnostic. The driver's `mutation_hook` invokes this wrapper. Document `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12`. (architect-r2 3, logic-r3 5)

- `test_database_is_disposable` connects to the wrapper's database, asserts `current_database() = 'lgtm_store_test'`, checks its owner and the login's database-creation privilege, and creates a scratch database. Inspect loopback publication and absence of the disposable password from command arguments and logs. Confirm cleanup after child failure and a handled interrupt. (architect-r3 5)
- `mutation_hook_passes_through_server_status` tests a missing executable and an input fixture that returns known success and failure statuses. Check the inherited setting without printing it.

This item is risky. Mutate the database name or publish its port beyond loopback. The database or port assertion must fail. Acceptance is `bash deploy/test/vault-fixtures.sh` with both assertions passing. Use disposable inputs directly without a shared fault-injection fixture. (logic-r3 5)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| Development mode could bypass PostgreSQL. | High | Require row growth and the original value after restart. |
| Test access could reach production data. | High | Publish a disposable fixture on loopback and tear it down on exit. |

**Verify.** Run the proof and `bash deploy/test/vault-fixtures.sh` on Linux Docker. After Server merges, run PB5 with Go. No device check is needed.

**WS-2: Start the two services with their original credentials**

Build and verify each service before the full launcher exists. Use the acceptance command in each item as its full gate. (architect-r2 9)

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 2a | Both logins authenticate and own separate databases. Neither login opens the other's database. | `postgres_roles_are_isolated` and `postgres_waits_for_both_databases`. |
| 2b | An interrupted role setup resumes with the old volume. The existing rollout helper reads the application database. | `postgres_repairs_half_initialized_volume` and `postgres_check_driver_authenticates`. |
| 2c | Key and storage checks preserve existing vault data. The daemons publish no host ports. | `seal_key_is_created_once`, `missing_key_preserves_existing_vault`, `wrong_key_preserves_existing_vault` and `vault_has_no_host_port`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 2 | Start PostgreSQL with authenticated role setup. | `deploy/compose.yaml`, `services.postgres` and `volumes.pgdata`; `deploy/stack-prepare.sh`, `prepare-db` and `reconcile-db`. |
| 3 | Prepare the seal and start OpenBao. | `deploy/openbao.hcl`, `deploy/openbao-start.sh` and `deploy/stack-prepare.sh seal`. |

**Work item 2. Start PostgreSQL and reconcile both service logins**

Create `deploy/compose.yaml`, `deploy/.gitignore`, `deploy/postgres-init.sql`, `deploy/stack-prepare.sh` and `deploy/test/postgres.sh`. Update the whole `psql_in_stack` function in `deploy/test/checks.sh` and its account and database entry in `deploy/test/README.md`. Stack writes all of these. Item 3 extends the Compose and prepare files after item 2 merges. (architect-r2 1, architect-r3 8)

- Use `postgres:16.16-alpine@sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d`. The default Compose project is `lgtm`. Use the project-scoped volume `pgdata` and `PGDATA=/var/lib/postgresql/data/pgdata`. The state root defaults to `deploy/.state` and can be overridden with an absolute `LGTM_STACK_STATE_DIR`. Prepare it at mode `0700`. Use long file binds with `create_host_path: false`. (architect-r3 2, logic-r3 2, logic-r3 8)
- `stack-prepare prepare-db` prepares the data directory for user and group 70. It makes `bao/`, `admin/` and `server/` under the state root, each owned by root with mode `0700`. Retain valid password files. Fill an absent file only for a fresh cluster. For a retained cluster with invalid or absent files, print `restore the matching PostgreSQL password files`. Each is an independent 32-byte value written as 64 lowercase hexadecimal characters and one newline. Use private temporary writes. Existing file and authentication checks detect mismatch without an installation identifier. (architect-r3 2, logic-r3 2)
- Run the official PostgreSQL entrypoint as 70:70. Preserve the volume with `restart: unless-stopped`. Set `POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 --auth-local=scram-sha-256`, `POSTGRES_HOST_AUTH_METHOD=scram-sha-256` and `POSTGRES_PASSWORD_FILE`. Salted Challenge Response Authentication Mechanism (SCRAM) applies to network and socket connections. Run `stack-prepare reconcile-db` as 70:70 through the authenticated final network listener. Require `pg_hba_file_rules` to contain only valid SCRAM rules. Existing invalid credentials or rules stop setup with a fixed diagnostic. (logic-r1 4, threat-r2 3, logic-r2 8)
- Reconciliation creates missing logins and databases `lgtm_app` and `lgtm_bao`, each with the login as database owner. Both logins lack administrative and database-creation privileges. Remove public connect on the service databases, `postgres` and `template1`, and prevent public schema creation. Recheck ownership and authenticate each role over the network on every reconciliation. The Compose health command runs `SELECT 1` for both roles. A wrong password on either account fails readiness. (architect-r1 5, logic-r1 1, logic-r2 7)
- Mount the three password files in PostgreSQL under `/run/lgtm-db/` by basename. Keep their values out of command arguments and ordinary logs. The test runner uses the command in the sprint 2 contract below. (architect-r3 8)

| Password file | Owner and group, mode | Writer and readers |
|---|---|---|
| `postgres-admin.password` | 70:70, `0400`. | `prepare-db` writes. The image and reconciler read. |
| `postgres-app.password` | 70:10001, `0440`. | `prepare-db` writes. Reconciliation, readiness, the rollout query helper and later Server read. |
| `postgres-bao.password` | 70:10002, `0440`. | `prepare-db` writes. Reconciliation, readiness, the seal probe and OpenBao read. |

- `postgres_roles_are_isolated` uses a client on the data network. Both role owners can create a table in their own database and neither can connect to the other's. Verify that the database daemon publishes no port.
- `postgres_waits_for_both_databases` checks unready before reconciliation. Substitute each mounted role password separately. Require failed health and affected login for each and restored health with the originals.
- `postgres_has_no_trust_rules` verifies that a disposable volume created without the flags cannot reconcile. It must refuse before role setup.
- `postgres_repairs_half_initialized_volume` retains an application row and its original password after completing missing role setup.
- `postgres_check_driver_authenticates` uses the real `psql_in_stack` on the fixture. Provide a query through standard input and require the application user and database in the result. The wrong fixture file must fail. Share the actual function without running the rest of the rollout driver. (architect-r2 1, architect-r3 8)

This item is risky. Remove authentication flags or have health probe one role only. Require the affected assertion to fail. Acceptance is `bash deploy/test/postgres.sh` with all five assertions passing.

**Work item 3. Prepare the key and start OpenBao**

Create `deploy/openbao.hcl`, `deploy/openbao-start.sh` and `deploy/test/seal.sh`. Extend `deploy/compose.yaml` and `deploy/stack-prepare.sh`. Use `openbao/openbao:2.7.3@sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9` for the daemon. Item 4 reuses this same digest for the setup job after this item merges. (architect-r3 4, architect-r3 7)

- `stack-prepare seal` authenticates as `lgtm_bao` and checks for rows in `public.openbao_kv_store` when present. Any probe error stops setup. If vault rows, the saved initialization response or a published credential exist, an absent key returns `seal key missing for existing vault`. Preserve a regular, nonsymlink 32-byte key owned by 10002:10002 at mode `0400`. A malformed file returns `seal key invalid` with a restore diagnostic. (architect-r3 2, logic-r3 2, logic-r2 5)
- The preparer writes `bao/unseal.key` once, and OpenBao reads it at `/run/lgtm-bao/unseal.key` through a read-only mount. Use `seal "static"` with key identifier `lgtm-1`. Use `storage "postgresql"` and a connection to the database `lgtm_bao` with `sslmode=disable`. The explicit `/bin/sh /openbao-start.sh server` entrypoint reads the password file and runs `bao server -config=/openbao/config/lgtm.hcl`. It bypasses the image's default development command. Set `BAO_ADDR` to loopback inside the daemon for the existing `vault_ready` probe.
- The database joins internal network `data`. OpenBao joins `data` and `vault`, listens on its container interfaces on port 8200, and waits for healthy PostgreSQL. Neither daemon publishes a host port. Publish only the disposable database on host loopback in sprint 1. An internal bridge can reach host listeners on its bridge address, so keep sensitive listeners off those addresses. (threat-r2 4)

| Sprint 1 process | User and group | Added capabilities after dropping all |
|---|---|---|
| PostgreSQL, reconciliation and seal database probe | 70:70. | None. |
| OpenBao daemon | 10002:10002. | None. |
| Offline private-file preparation | 0:0. | `CHOWN`, `DAC_OVERRIDE`, `FOWNER` for protected state and file ownership. No network. |
| OpenBao setup job in item 4 | 0:0. | `CHOWN` and `DAC_OVERRIDE` to prepare and validate protected credential files. Only the vault network. |

Stack declares these settings with `no-new-privileges:true`. The retained service checks read the effective users and capability sets. (architect-r2 8, logic-r2 8, threat-r2 8)

- `seal_key_is_created_once` verifies mode, owner and unchanged bytes on repeat.
- `missing_key_preserves_existing_vault` uses a disposable service fixture with a stored record. Move the key aside using the fixture's normal Docker access. Startup refuses without creating a new key, and the original still opens the record. Repeat the refusal with local bootstrap evidence and no vault rows.
- `wrong_key_preserves_existing_vault` tests a different valid-length key and a malformed file in separate runs. The wrong key stays sealed. The malformed file reports `seal key invalid`. Preserve rows in both runs.
- `vault_has_no_host_port` verifies both services' port configuration, OpenBao's PostgreSQL storage status, the network membership and the declared and effective identities and capability sets for the daemons and earlier jobs.

This item is risky. Bypass the key guard or use the default development command and require the affected test to fail. Acceptance is `bash deploy/test/seal.sh` with all four assertions passing. Use ordinary disposable Compose projects. (logic-r3 5)

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A partial PostgreSQL setup leaves one role unusable. | High | Verify both roles on the network and test interrupted reconciliation. |
| A lost key would make retained secrets unreadable. | High | Refuse automatic substitution when storage or bootstrap material exists. |

**Verify.** Run `bash deploy/test/postgres.sh` and `bash deploy/test/seal.sh` against Linux Docker. No device check is needed.

**WS-3: Give the server a restricted vault credential and restart the stack**

The setup job publishes a credential only after it works. The launcher repeats the check with the same files and starts again after shutdown.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3a | The published file signs in and reaches the signing-key path only. A repeat leaves the same files and role in place. | `vault_publishes_scoped_credential` and `stack_starts_twice_without_changes`. |
| 3b | A full stack restart keeps the vault open and the signing-key path readable. | `stack_reopens_existing_vault`. Full C13 in sprint 2 uses `deploy/test/checks.sh --only C13`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 4 | Initialize the vault and publish the restricted file. | `deploy/openbao-init.sh`, `bao operator init` and `server/openbao.json`. [OpenBao setup sources](scouting.md#round-3-sources-and-current-anchors). |
| 6 | Start and restart the sprint 1 stack. | `deploy/up.sh` and `deploy/test/stack-vault.sh`, following the existing driver's `check_c1`, `vault_ready` and `check_c13`. |

**Work item 4. Initialize OpenBao and publish one server credential**

Create `deploy/openbao-init.sh`, `deploy/openbao-server-policy.hcl` and `deploy/test/vault-init.sh`. The one-shot Compose service `openbao-init` uses the daemon's pinned image under the `setup` profile, joins only `vault` and mounts only `admin/`, `server/` and the read-only policy. Disable tracing and keep secrets out of arguments, Compose environments, inspection and logs. (architect-r3 4, architect-r3 7, logic-r3 5)

The initializer writes and rereads both files. The seal guard checks their presence. Server reads only the published file. (architect-r3 3, logic-r3 4)

| Fact | File and access |
|---|---|
| Initialization response saved | Save raw `bao operator init -format=json -recovery-shares=1 -recovery-threshold=1` output at `admin/init-response.json`. Use owner and group 10002 and mode `0400`. Only local setup mounts this administrator file. (architect-r3 1, logic-r3 1, architect-r3 6) |
| Server credential published | Write and flush a private temporary file with the sprint 2 format and ownership. Rename it to `server/openbao.json` in the same directory and flush the directory. Only the final name counts. (architect-r3 3, logic-r3 4, logic-r3 5) |

- Check initialization separately from the single 60-second elapsed-time active wait. If uninitialized with either file, report `state directory is ahead of the database`. If initialized without a valid response, report `vault is initialized but bootstrap state is incomplete`. On a wait failure report `OpenBao did not become active`. Preserve both files on refusal and flush the saved response before authenticated changes.
- **UNREVIEWED FIX (logic-r4 1).** In the job, use BusyBox `sed` to extract one `root_token` from the complete pinned init output shape. Reject malformed or truncated output as incomplete. Supply `BAO_TOKEN` only to authenticated `bao` child processes. Get `role_id` via `bao read -field=role_id` and `secret_id` via `bao write -f -field=secret_id`. Use shell `printf` after checking the generated identifiers' character set. Feed the protected file to `bao write auth/approle/login -` and discard the token-bearing output. Only tests use host `jq`.
- With the response saved and no published file, enable version 2 of the key-value engine at `lgtm` and application-role authentication (AppRole) at `auth/approle`. Reuse matching mounts. Apply policy and role `lgtm-server`, granting read, create and update at `lgtm/data/server/link/signing-key`. Set `secret_id_ttl=0`, `secret_id_num_uses=0`, `token_type=batch`, `token_ttl=15m`, `token_max_ttl=15m` and `token_no_default_policy=true`. (architect-r3 3, logic-r3 4)
- **UNREVIEWED FIX (logic-r4 2).** Require a successful read of `auth/approle/role/lgtm-server` after applying the role. Only then does `bao list -format=json auth/approle/role/lgtm-server/secret-id` with exit 2, exactly `{}` on standard output and empty standard error permit one issuance. This is the pinned client's empty-list response. Existing accessors, a failed role read or any other list outcome stop issuance with the incomplete-setup diagnostic and preserve evidence.
- Validate login before publishing. With a final file, validate once without provisioning or rewriting either file. An unusable file returns `server vault credential needs repair`.

Freeze `vault_publishes_scoped_credential` in `deploy/test/vault-init.sh`. Check file modes, owners and fields, a 900-second login lease and a signing-key round trip. Deny sibling, policy, tooling and unauthenticated administrator access. Check a missing response and the PB6 and PB7 cases on the real daemon. (architect-r3 1, architect-r3 3, logic-r3 4, logic-r3 5)

This item is risky. Broaden the policy or publish an invalid identifier and require the denial or login assertion to fail. Acceptance is `bash deploy/test/vault-init.sh` with its named assertion passing.

**Contract for sprint 2**

| Surface | Writer or supplier | Reader |
|---|---|---|
| Runtime user | The Stack web plan runs the server image as numeric user and group 10001. | The Server program reads mounted files as that identity. |
| Database | [Stack web item 4](../go-s2-stack-web/plan.md) sets `LGTM_POSTGRES_HOST=postgres`, `LGTM_POSTGRES_PORT=5432`, `LGTM_POSTGRES_DATABASE=lgtm_app`, `LGTM_POSTGRES_USER=lgtm_app` and `LGTM_POSTGRES_PASSWORD_FILE=/run/lgtm/postgres.password`. Mount `postgres-app.password` read-only there. | [Server sign-in item 1](../go-s2-server-signin/plan.md) reads the five settings and the hexadecimal password file. (architect-r3 8, logic-r3 10) |
| Rollout database query | Item 2 owns `psql_in_stack` and its test guide. Run `compose exec -T --user 70:70 postgres` and read `/run/lgtm-db/postgres-app.password` into `psql -w -h 127.0.0.1 -U lgtm_app -d lgtm_app`. Keep bound arguments and standard input. | C5 uses the helper in sprint 3. (architect-r3 8) |
| Vault credential | Stack writes `server/openbao.json` under `deploy/.state` or the override. Use 8-bit Unicode Transformation Format (UTF-8) JavaScript Object Notation (JSON) with exactly two nonempty strings, `role_id` and `secret_id`, and one trailing newline. Use owner and group 10001 and mode `0400`. Stack sets `LGTM_OPENBAO_CREDENTIAL_FILE` to the read-only mount `/run/lgtm/openbao.json` and `LGTM_OPENBAO_ADDR` to `openbao` on port 8200 over plain Hypertext Transfer Protocol (HTTP). | Server reads both settings, uses `POST /v1/auth/approle/login` and sends the returned `auth.client_token` in `X-Vault-Token`. |
| Signing key and observation | Server writes its 32-byte Ed25519 seed as standard base64 in field `private_seed_b64` at `lgtm/data/server/link/signing-key`. The first write uses `options.cas=0`. Server emits `msg=link_signing_key_loaded`, `source=openbao` and a nonempty public string `fingerprint`. | Server reads the stored seed after a creation conflict. `server_key_fingerprint` in the rollout driver reads the log fields for C13. (threat-r2 10) |
| Launch and networks | Item 6 writes `deploy/up.sh` and supplies private `data` and `vault` networks. | Web extends the launcher after vault success, adds its `--until vault` fallback and updates `deploy/test/stack-vault.sh`. (architect-r3 8) |
| Web migration. **UNREVIEWED FIX (logic-r4 3).** | Sprint 1 has no writer for `deploy/test/vault-testlib.sh`, `fixture_create`, `fixture_cleanup`, `--reset --yes` or `LGTM_POSTGRES_URL`. | Web owns disposable Compose projects, state, loopback ports and teardown per test. Drop `up_reset_removes_web_and_server` and use the five database settings. Replace the old PB5 trace for `up_starts_no_server_before_the_vault` with the vault phase and PB8. Trace full restart to PB4 and C13. Sprint 1 item 2 owns C5. |

**Work item 6. Verify first start and restart of the stack**

Create executable `deploy/up.sh`, `deploy/README.md` and `deploy/test/stack-vault.sh`. Take an exclusive `flock` on the canonical state directory, one per Compose project. Emit `phase <name> start` and `phase <name> done` for the web launcher and rollout tests. (architect-r3 2, logic-r3 2)

| Phase | Operation |
|---|---|
| `database` | Run `prepare-db`, start `postgres`, run `reconcile-db` and require authenticated health for both logins. |
| `seal` | Run `seal` before starting OpenBao. Stop on a failed key or storage check. |
| `vault` | Start `openbao`, run `openbao-init`, and require both daemons healthy. |
| `ready` | Complete launcher readiness. After the last phase line, print `postgres and openbao are healthy`. A failed phase exits nonzero with its diagnostic. |

Document Docker Compose for startup and ordinary `down` for shutdown. Use `docker compose -f deploy/compose.yaml` with existing project and state settings. For manual policy administration use the local administrator response. Keep it out of logs, shell history and server mounts. (architect-r3 1, logic-r3 1)

For a known unfinished disposable installation, confirm the project and state folder. Run `docker compose -f deploy/compose.yaml -p <project> down -v`, remove that project's state folder as its owner or an administrator, and relaunch. Deleting either side can leave data unusable. Preserve both sides after the stack has held data. Rollback is outside wave 1. (architect-r3 2, logic-r3 3, logic-r3 6, logic-r3 7, logic-r3 9)

- `stack_starts_twice_without_changes` runs the launcher twice. Both runs reach ready with one identifier and two healthy daemons. Compare the fingerprints and modification times of the administrator, seal and server files.
- `stack_reopens_existing_vault` writes a fixture signing key using the published file as user 10001. After ordinary shutdown and relaunch with the same state and volume, OpenBao unseals automatically and the file reads the same value. This checks the vault part of C13.

This item is risky. Skip database readiness or provision on repeat start and require failure. Run `bash deploy/test/stack-vault.sh` for two passing assertions. Full C13 waits for sprint 2. (architect-r3 2, architect-r3 3, logic-r3 5)

**Post-build checks**

| Check | Scenario and expected outcome |
|---|---|
| PB1. Clean Linux launch | Start on a clean checkout with the two pinned images and no cached helper. Verify exact service users and capabilities, long file binds, and no user-entered unseal key. Record platform and digests. (architect-r3 4) |
| PB2. Unfinished initialization | On a disposable project, initialize with the normal pinned command and retain its protected response before running the regular launcher. Expect one published credential and the reviewed policy. The second run leaves them unchanged. Missing the response after initialization must leave the database intact and name the incomplete state. (architect-r3 3, logic-r3 4) |
| PB3. Secret handling | Inspect arguments, logs, rendered Compose and container configuration during success and rejected password runs. Require no fixture secrets. Check the two durable bootstrap files are the only bootstrap records and that the administrator file is never exposed to Server. (architect-r3 1, logic-r3 1) |
| PB4. Restart and concurrency | Run two launchers against the same state directory, including a symbolic-link spelling. The lock serializes them, then both may reach ready with one credential. After a host restart run C13 when the server is present. The signing-key fingerprint must match. (architect-r3 2) |
| PB5. Database rollout hooks | With Server's store merged, run `deploy/test/with-postgres.sh -- go test ./src/server/internal/store/...` and `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12`. The maintenance name check, database suite and expected change-feed mutation all pass. The C5 query helper independently reads the application database. (architect-r3 5) |
| PB6. Setup tools and response | In the pinned image without `jq` or external network access, reach OpenBao on `vault` and publish one credential. A truncated init response that still contains the token fails with `vault is initialized but bootstrap state is incomplete` and leaves state intact. Mutating the job to call `jq` must fail. (logic-r4 1) |
| PB7. Empty identifier list | Record the nonsecret list result for a fresh role and require zero accessors to become exactly one. One existing accessor with no final file, a failed role read or a list error each forbid issuance. Treating the pinned empty response as an error must fail first start. (logic-r4 2) |
| PB8. Web launcher handoff | With Web's disposable projects, run `up_starts_everything_in_order`, `up_until_vault_stops_early` and `up_starts_no_server_before_the_vault`. Full launch uses the five Server settings. Early stop and a malformed vault credential both leave `server` and `web` absent. The malformed credential produces `server vault credential needs repair`. (logic-r4 3) |

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A host reader can administer the whole vault if it can read the initialization response. | High | Protect that file the same way as the unseal key and mount it only for local setup. |
| An interrupted first setup can leave no usable response or published credential. | High | Leave both stores intact and document manual cleanup for a known unfinished installation. |

**Verify.** Run `bash deploy/test/vault-init.sh` and `bash deploy/test/stack-vault.sh` on Linux Docker. Freeze the named assertions before building. The lead checks the exact commit and the merged result, including the named mutations. No device check is needed.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe comprises D4, D9, D10, R9, R10, the vault storage, bootstrap, name and service boundary clauses, the database fixture in C5, C12 and C13, and the four work items of the original brief. [Foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md), [original brief](../../../.foundry/go-rewrite/tasks/plan-go-s1-stack-vault.md).

| Source item | Workstream | Evidence |
|---|---|---|
| D4, PostgreSQL in Compose | WS-1, WS-2 | Items 1, 8 and 2 prove, fixture and start real storage. |
| D9, server secrets in a vault | WS-3 | Item 4 publishes a credential for the server's path. |
| D10, OpenBao beside PostgreSQL | WS-1, WS-2, WS-3 | Items 1, 3 and 6 show storage and startup. |
| Vault storage and seal | WS-1, WS-2 | Items 1 and 3 verify row growth and key behavior. |
| First setup and restricted credential | WS-3 | Item 4 verifies login before publication. |
| Secret naming and service boundary | WS-2, WS-3 | Items 3 and 4 use internal networks and an exact policy. |
| R9, no secrets in user surfaces | WS-1, WS-2, WS-3 | Tests check protected files and private channels. |
| R10, real PostgreSQL tests | WS-1, WS-2 | Items 1, 8 and 2 run real database queries. |
| C5, fixture database query | WS-2 | The retained driver helper authenticates in item 2. |
| C12, database suite and change-feed mutation | WS-1 | Item 8 matches the Server fixture contract. |
| C13, restart and signing key | WS-1, WS-2, WS-3 | Items 1, 3 and 6 show auto-unseal. Sprint 2 verifies the server fingerprint. |
| Brief item 1 | WS-1 | Item 1 gates the production build on the disposable proof. |
| Brief items 2 and 3 | WS-2 | Items 2 and 3 provide authenticated storage and the vault. |
| Brief item 4 | WS-2, WS-3 | Items 3, 4 and 6 prepare the key, publish the credential and check first start. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository rule | Evidence |
|---|---|
| `AGENTS.md`, product parts and contract | Stack supplies protected files. Server owns the vault adapter. |
| `AGENTS.md`, known source locations, and the epic's Stack scope | Production and retained test changes go in `deploy/`. The planner records the disposable proof receipt. |
| `AGENTS.md`, habit 1, credits copied code | Use upstream commands and original setup scripts. |
| `AGENTS.md`, habit 2, records problems | The round registers and scouting record verified findings and Docker availability. |
| `AGENTS.md`, habit 3, routes finished changes through a pull request | Verify the exact commit before the base-branch handoff. |
| `AGENTS.md`, writing style | Scan the artifacts and keep proof receipts free of private details. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Reason | Attack pass | Depends on | Can run alongside | Design statements |
|---|---|---|---|---|---|---|---|---|
| 1. Image proof | `infra` | `stack/infra` | medium | It observes stored data and restart behavior on both daemons. | Yes | None. | None. | D4, D10, R9, R10, C13. |
| 8. Disposable database and hook | `infra` | `stack/infra` | low | Two launch wrappers have independent input and exit assertions. | Yes | Item 1. | None. | D4, R9, R10, C12. |
| 2. PostgreSQL | `infra` | `stack/infra` | medium | It prepares credentials and reconciles two database owners. | Yes | Items 1 and 8. | None. | D4, D10, R9, R10, C5. |
| 3. Seal and OpenBao | `infra` | `stack/infra` | medium | It starts one daemon and preserves its existing storage and key. | Yes | Item 2. | None. | D10, R9, C13. |
| 4. First-start vault setup | `infra` | `stack/infra` | medium | It saves initialization and publishes a single authenticated credential. | Yes | Item 3. | None. | D9, D10, R9, C13. |
| 6. Launcher | `infra` | `stack/infra` | medium | It orders prepared phases and verifies ordinary restart. | Yes | Items 2, 3 and 4. | None. | D4, D10, R9, C13. |

Load. Six test writers, six builders and six mutation checks give 18 execution seats. Dividing them equally between the two primary vendors gives nine each. The orchestrator checks remaining weekly allowances before dispatch and counts review seats separately.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | Findings | Outcome |
|---|---|---|
| Round 1 | [Register](attack/round-1-findings.md). | Findings folded. |
| Round 2 | [Register](attack/round-2-findings.md). | Findings folded. |
| Round 3 | [Register](attack/round-3-findings.md). | The scope ruling supersedes earlier mechanisms. |
| Round 4 | [Register](attack/round-4-findings.md). | Three fixes go directly to build. |

</details>
