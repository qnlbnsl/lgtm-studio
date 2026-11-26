---
topic: go-s1-stack-vault
status: attack
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# PostgreSQL and OpenBao start together and reopen the vault after restart

## Overview

Sprint 1 gives LGTM Studio a database and a vault. One command starts both. OpenBao keeps its data in a separate PostgreSQL database and uses the same local key file after a restart.

Reviewers observed PostgreSQL storage and automatic unseal in disposable containers. The first build item repeats the proof with the production database role and container user. The sprint 2 server receives a protected credential that reaches only its signing-key path.

## Key decisions

The orchestrator reduced the plan to six work items on 2026-11-26. First start saves one protected administrator file and publishes the separate server credential after it can log in. A completed run verifies the existing credential and leaves both files alone. (architect-r3 1, architect-r3 3, logic-r3 1, logic-r3 4)

The launcher locks its state directory. Password and seal checks stop a start with mismatched files. A known unfinished first installation can be rebuilt manually from empty state. After an installation has served data, restore the matching state and volume. (architect-r3 2, logic-r3 2, logic-r3 3)

[Full decision log](decisions.md)

## Workstreams

### Prove storage and give the Server team a test database

The proof checks that writing to the vault adds PostgreSQL records. It replaces both services and reads back the same value. The separate test wrapper gives Server a disposable database for Go tests and the change-feed mutation.

### Start the two services with their original credentials

Each daemon uses its own database login. The launcher checks PostgreSQL before preparing the seal. Existing vault data or initialization files prevent automatic replacement of a missing key.

### Give the server a restricted vault credential and restart the stack

First start creates the signing-key policy and role. The launcher checks that the server credential can sign in before publishing its file. On restart OpenBao reopens automatically, and the same credential reads the original value.

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

Create `deploy/openbao-init.sh`, `deploy/openbao-server-policy.hcl` and `deploy/test/vault-init.sh`. Extend Compose with the one-shot service `openbao-init` under the `setup` profile. Use the daemon's pinned OpenBao image and its `bao` command-line interface. The job joins only `vault` and mounts only `admin/`, `server/` and the read-only policy. Use the host's existing `jq` prerequisite for structured output. Transfer administrator and server values through protected streams or files, with tracing disabled. Docker arguments, service environments, inspection and logs must contain no secret values. (architect-r3 4, architect-r3 7, logic-r3 5)

Record only the two facts below. Stack's initializer owns the files. It reads them itself on retry. The seal guard reads their presence, and Server reads only the published file. (architect-r3 3, logic-r3 4)

| Fact | File and access |
|---|---|
| Initialization response saved | `admin/init-response.json` stores the output of `bao operator init -format=json -recovery-shares=1 -recovery-threshold=1`. Keep it owner and group 10002 and mode `0400`, like the unseal key. It is vault administrator material. Read it only through the local setup path. Mount it in neither daemon nor Server. (architect-r3 1, logic-r3 1, architect-r3 6) |
| Server credential published | `server/openbao.json` has the format and ownership in the sprint 2 contract. Publish with a private temporary file and ordinary rename inside the same destination directory. Flush the file and directory. Only the final filename counts as published. (architect-r3 3, logic-r3 4, logic-r3 5) |

- Check initialization and active status separately. An uninitialized vault with either retained file is `state directory is ahead of the database`. An initialized vault without a valid administrator response is `vault is initialized but bootstrap state is incomplete`. Preserve the state on either refusal. Save and flush the one-time initialization response before any authenticated change. Use one 60-second active-health wait measured by elapsed time. A wait failure prints `OpenBao did not become active` and leaves both files intact.
- With the response saved and no published file, enable version 2 of the key-value engine at `lgtm` and OpenBao's application-role authentication, called AppRole, at `auth/approle`. Reuse matching mounts on a retry. Apply the exact policy and role `lgtm-server`. Grant read, create and update at `lgtm/data/server/link/signing-key`. Set `secret_id_ttl=0`, `secret_id_num_uses=0`, `token_type=batch`, `token_ttl=15m`, `token_max_ttl=15m` and `token_no_default_policy=true`. Issue one identifier only when none exists for the role. If issuance was interrupted and an identifier already exists without a published file, report incomplete setup and preserve the evidence. A read or authorization error never counts as absence. Use the operator guide for a known unfinished first installation. (architect-r3 3, logic-r3 4)
- Validate the server login before publishing. With a valid published file, perform one validation login and skip initialization and provisioning. Leave the policy, role and both files unchanged. An invalid or unusable final file returns `server vault credential needs repair`. The launch creates no substitute. The recovery guide distinguishes an unfinished first start from an installation with data.

Freeze `vault_publishes_scoped_credential` in `deploy/test/vault-init.sh`. Require the administrator and published file modes, expected owner and fields, and a 900-second login lease. Write and read a disposable signing-key value. Deny a sibling secret, administrative policy and tooling path. An initialized fixture with a deliberately missing response must return the incomplete-setup diagnostic. Verify an unauthenticated request cannot obtain administrator access. Use the real daemon without a recording proxy or command shim. (architect-r3 1, architect-r3 3, logic-r3 4, logic-r3 5)

This item is risky. Broaden the server policy or attempt publication with an invalid identifier. Require the denial or login assertion to fail. Acceptance is `bash deploy/test/vault-init.sh` with the named assertion passing.

**Contract for sprint 2**

| Surface | Writer or supplier | Reader |
|---|---|---|
| Runtime user | The Stack web plan runs the server image as numeric user and group 10001. | The Server program reads mounted files as that identity. |
| Database | Stack sets `LGTM_POSTGRES_HOST=postgres`, `LGTM_POSTGRES_PORT=5432`, `LGTM_POSTGRES_DATABASE=lgtm_app`, `LGTM_POSTGRES_USER=lgtm_app` and `LGTM_POSTGRES_PASSWORD_FILE=/run/lgtm/postgres.password`. Mount `postgres-app.password` read-only there. | [Server sign-in work item 1](../go-s2-server-signin/plan.md) consumes these five settings and the hexadecimal password file. [Stack web work item 4](../go-s2-stack-web/plan.md) supplies the same names when it mounts the server. (architect-r3 8, logic-r3 10) |
| Rollout database query | Item 2 replaces the entire `psql_in_stack` connection logic and removes the old account and database defaults from the test guide. Run `compose exec -T --user 70:70 postgres`. Inside it, read `/run/lgtm-db/postgres-app.password` into the local `psql` process and use `psql -w -h 127.0.0.1 -U lgtm_app -d lgtm_app`. Keep bound arguments and standard input. | C5 uses this command in sprint 3. The web plan leaves the helper to item 2. (architect-r3 8) |
| Vault credential | Stack writes default `deploy/.state/server/openbao.json` or `server/openbao.json` under the state override. It is 8-bit Unicode Transformation Format (UTF-8) JavaScript Object Notation (JSON) with exactly two nonempty string fields, `role_id` and `secret_id`, and one trailing newline. Use owner and group 10001 and mode `0400`. Stack mounts it read-only at `/run/lgtm/openbao.json` and sets `LGTM_OPENBAO_CREDENTIAL_FILE` to that path. Stack sets `LGTM_OPENBAO_ADDR` to the `openbao` service on port 8200 over plain Hypertext Transfer Protocol (HTTP). | Server reads those two settings. AppRole login uses `POST /v1/auth/approle/login`. Server reads `auth.client_token` and sends it in `X-Vault-Token`. |
| Signing key and observation | Server writes its 32-byte Ed25519 seed as standard base64 in field `private_seed_b64` at `lgtm/data/server/link/signing-key`. The first write uses `options.cas=0`. Server emits `msg=link_signing_key_loaded`, `source=openbao` and a nonempty public string `fingerprint`. | Server reads the stored seed after a creation conflict. `server_key_fingerprint` in the rollout driver reads the log fields for C13. (threat-r2 10) |
| Launch and networks | Item 6 writes executable `deploy/up.sh` and supplies the private `data` and `vault` networks. | The web plan extends the launcher and adds the server only after successful vault setup. The launcher option `--until vault` belongs to the web plan's fallback. It adds the option there and updates the surviving `deploy/test/stack-vault.sh` invocation. (architect-r3 8) |

**Work item 6. Verify first start and restart of the stack**

Create executable `deploy/up.sh`, `deploy/README.md` and `deploy/test/stack-vault.sh`. Hold the existing exclusive `flock` on the canonical state directory during startup. Use one state directory per Compose project. The launcher emits `phase <name> start` and `phase <name> done`. Stack writes these observations, and the web launcher and rollout tests consume them. (architect-r3 2, logic-r3 2)

| Phase | Operation |
|---|---|
| `database` | Run `prepare-db`, start `postgres`, run `reconcile-db` and require authenticated health for both logins. |
| `seal` | Run `seal` before starting OpenBao. Stop on a failed key or storage check. |
| `vault` | Start `openbao`, run `openbao-init`, and require both daemons healthy. |
| `ready` | Complete launcher readiness. After the last phase line, print `postgres and openbao are healthy`. A failed phase exits nonzero with its diagnostic. |

Use `docker compose -f deploy/compose.yaml` with the same project and state settings. Ordinary shutdown uses `down` without volume removal. In `deploy/README.md`, name Docker Compose and `jq` as startup prerequisites. Explain that `admin/init-response.json` holds the local administrator token for any manual policy administration. Keep it out of logs, shell history and server mounts. (architect-r3 1, logic-r3 1)

Document a rebuild only for an operator who knows first start never succeeded and whose stack is still disposable. Confirm the project and its dedicated state folder, remove that project's volumes with `docker compose -f deploy/compose.yaml -p <project> down -v`, remove that project's state folder using its owner or an administrator, and run the launcher again. Document that deleting either side alone can make existing data unusable. If the stack has held data, preserve both sides and restore matching files and volume from backup. The launcher performs no destructive recovery. (architect-r3 2, logic-r3 3, logic-r3 6, logic-r3 7, logic-r3 9)

- `stack_starts_twice_without_changes` runs the real launcher twice and compares the administrator file, seal file and server credential by fingerprint and modification time. Compare the role's identifier count. Both runs reach ready with one identifier and two healthy daemons.
- `stack_reopens_existing_vault` writes a fixture signing-key value with the published file as user 10001. Run plain Compose shutdown and rerun the launcher with the same state and volume. The vault unseals without input and the same file reads the original value. This is the vault half of C13.

This item is risky. Bypass database readiness or provision on an already completed run and require a startup assertion to fail. Acceptance is `bash deploy/test/stack-vault.sh` with two passing assertions. The full check waits for sprint 2's Go server and website. (architect-r3 2, architect-r3 3, logic-r3 5)

**Post-build checks**

| Check | Scenario and expected outcome |
|---|---|
| PB1. Clean Linux launch | Start on a clean checkout with the two pinned images and no cached helper. Verify exact service users and capabilities, long file binds, and no user-entered unseal key. Record platform and digests. (architect-r3 4) |
| PB2. Unfinished initialization | On a disposable project, initialize with the normal pinned command and retain its protected response before running the regular launcher. Expect one published credential and the reviewed policy. The second run leaves them unchanged. Missing the response after initialization must leave the database intact and name the incomplete state. (architect-r3 3, logic-r3 4) |
| PB3. Secret handling | Inspect arguments, logs, rendered Compose and container configuration during success and rejected password runs. Require no fixture secrets. Check the two durable bootstrap files are the only bootstrap records and that the administrator file is never exposed to Server. (architect-r3 1, logic-r3 1) |
| PB4. Restart and concurrency | Run two launchers against the same state directory, including a symbolic-link spelling. The lock serializes them, then both may reach ready with one credential. After a host restart run C13 when the server is present. The signing-key fingerprint must match. (architect-r3 2) |
| PB5. Database rollout hooks | With Server's store merged, run `deploy/test/with-postgres.sh -- go test ./src/server/internal/store/...` and `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12`. The maintenance name check, database suite and expected change-feed mutation all pass. The C5 query helper independently reads the application database. (architect-r3 5) |

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
| Round 1 | [Register](attack/round-1-findings.md). | The round has a recorded disposition for every finding. |
| Round 2 | [Register](attack/round-2-findings.md). | The round has a recorded disposition for every finding. |
| Round 3 | [Register](attack/round-3-findings.md). | The orchestrator's scope ruling replaces the listed earlier mechanisms. The next round reviews only the changed headings. |

</details>
