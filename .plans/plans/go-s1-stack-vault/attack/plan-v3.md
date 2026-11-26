---
topic: go-s1-stack-vault
status: attack
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# PostgreSQL and OpenBao start together and keep the vault open after a restart

## Overview

Sprint 1 gives LGTM Studio a database and a vault that can run on their own. One command starts both services. PostgreSQL keeps the application's records and a separate database for the vault. OpenBao uses the same key file after a restart, so routine startup needs no manual unseal.

Reviewers observed both PostgreSQL storage and automatic unseal in disposable containers. The first build item repeats the proof with the production database role and container user. It checks that a missing or wrong key leaves the existing vault closed. (architect-r1 1, logic-r1 5, critic-r1 6)

The sprint 2 server reads its credential from a protected file. The credential can reach the server's signing-key path. A local maintenance credential gives the operator a way to replace it and update that policy after the initial administrator token is revoked. (threat-r1 1, logic2-r1 2)

## Key decisions

Installation keeps the vault key and the database together on the same machine. Startup stops when those files and the database belong to different installations. The operator can restore the matching local files. A separate reset command requires explicit confirmation and removes the two databases and the current installation's local state. (logic-r1 2, architect-r1 6, threat-r1 3)

The maintenance credential can indirectly change which secrets the server can reach. Protect it as vault administrator material. (architect-r2 7, threat-r2 1)

OpenBao's application-role authentication, called AppRole, exchanges the server's file credential for a token that lasts 15 minutes. Startup validates the published file with one login. The file, policy, and role stay in place on a normal rerun. (architect-r1 6)

[Full decision log](decisions.md)

## Workstreams

### Prove storage and restart behavior

The proof writes a sample secret and checks that PostgreSQL gains records. It replaces OpenBao after restarting PostgreSQL and reads back the same secret. Key controls distinguish a missing file from a valid-length file with different bytes. The fixture is removed when the run ends.

### Start PostgreSQL and open the vault inside the stack

Each service owns its database. Setup authenticates over PostgreSQL's network listener before it advances. It repairs partially created database roles and databases on a retry, using the original passwords. It reuses the original seal key whenever the vault contains data. (architect-r1 5, logic2-r1 4)

### Hand off a restricted server credential

First start records checkpoints so setup can resume after an interruption. It verifies that the original administrator token has ended before publishing the server credential. The tests use OpenBao directly, so they run before the Go server exists.

## Rollout order

1. Run the production-shape proof and teardown. Record the image digests and command output before releasing the later items.
2. Prepare a fresh installation and authenticate to each owned PostgreSQL database. Prepare the static key and start OpenBao.
3. Initialize the vault and publish the restricted file. Rerun startup and restart the stack. Verify that the original secret and credential persist.
4. Verify local credential replacement in the disposable stack. Hand the service and credential contracts to the sprint 2 plans.

---
<!-- ⛔ implementation details and gate evidence below -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: Prove storage and restart behavior**

The first check closes the gap between documented features and the exact images, role permissions and process identity used by the stack. In each workstream, the acceptance command in each item is its full gate. The Done when rows name representative observations. (architect-r2 9)

**Done when**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1a | The restricted database owner stores the sample secret in PostgreSQL. A fresh OpenBao process reads it after both services restart. | `postgres_storage_round_trip` and `static_seal_restart`. |
| 1b | A missing or wrong key leaves the same data closed. Restoring the original key opens it, and the project is removed. | `missing_seal_key_blocks_unseal`, `wrong_seal_key_stays_sealed` and `cleanup=true`. |
| 1c | The shared fixture library creates isolated projects. A separate disposable PostgreSQL accepts a host connection and database creation. | `two_fixture_projects_clean_up` and `test_database_is_disposable`. |

**Tasks**

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 1 | Run the disposable storage and static seal proof. | [Scouting](scouting.md#production-shape-proof-for-work-item-1), proof commands. The target is `deploy/openbao.hcl`, using `storage "postgresql"` and `seal "static"`. |
| 8 | Provide isolated fixtures and the host test database. | `deploy/test/vault-testlib.sh` and `deploy/test/with-postgres.sh`, following `deploy/test/checks.sh`, `run_check`, and `deploy/test/README.md`, `LGTM_TEST_POSTGRES`. |

**Work item 1. Run the disposable proof**

Keep `proof.sh`, `proof.test.sh` and `bao.hcl` in a temporary directory outside the checkout. Use the [production-shape proof](scouting.md#production-shape-proof-for-work-item-1). Stack writes the exact-commit check report and sends its sanitized transcript to the planner or orchestrator. The planner or orchestrator updates `scouting.md` and records the digests and platform. This handoff keeps the Stack write scope inside `deploy/`. (critic-r1 6)

Use an unprivileged `lgtm_bao` login that owns its database. Run OpenBao as user and group 10002 through the same explicit launcher and listener settings as item 3. Initialize through the Hypertext Transfer Protocol (HTTP) application programming interface (API). Record the image defaults separately. The reviewer transcript proves the upstream facts. The production-shape checks below gate items 2, 3 and 4. (architect-r1 1, logic-r1 5)

Freeze these assertions first in temporary `proof.test.sh`. A stub that leaves the row count unchanged, always reports sealed or returns no value makes the positive assertions fail.

- `postgres_storage_round_trip` checks that the restricted role creates `public.openbao_kv_store`, that the sample write increases its row count, and that the stored value survives a replacement of both services.
- `static_seal_restart` requires `initialized=true`, `sealed=false` and `storage_type=postgresql` within 60 seconds of restart. The test sends no unseal request.
- `missing_seal_key_blocks_unseal` expects a missing-file diagnostic and an unhealthy or exited process. It preserves the PostgreSQL rows and recovers the sample with the original key.
- `wrong_seal_key_stays_sealed` replaces the key with a different 32-byte value under the same identifier. It expects the vault to stay sealed or exit and verifies unchanged storage. Restoring the original key must recover the value.
- `bootstrap_root_generation_requires_authentication` records the refusal without an operator token, including after revocation of the original root token. The report contains status codes only. (threat-r1 1)

The item is risky. Changing storage to an in-memory backend must fail the row-growth assertion. Changing the positive restart key must fail the unseal assertion. Acceptance requires a named passing result for each assertion, image digests and `cleanup=true`. The orchestrator closes the image assumptions when the transcript passes. (architect-r1 1)

**Work item 8. Provide test fixtures and a disposable host database**

Create `deploy/test/vault-testlib.sh`, `deploy/test/with-postgres.sh`, `deploy/test/vault-fixtures.sh` and executable `deploy/test/mutate-feed-lock.sh`. Update `deploy/test/README.md`. These belong to Stack. The vault tests read the shared functions `fixture_create`, `fixture_postgres`, `fixture_openbao`, `fixture_exec_as` and `fixture_cleanup`. Stack also writes `fixture_key_action <stash|restore|fingerprint>` for items 3 and 6. The fixture uses a disposable privileged file helper on that fixture alone. It uses no production script from a later item. (architect-r2 2, critic-r1 4) Tests keep scenario-specific helpers in their own files. The shared library accepts a fixture-owned Compose override and uses unique project and state names with cleanup traps. It does not depend on an item 2 file. (critic-r1 8, logic-r1 9)

`deploy/test/with-postgres.sh -- <command>` starts an independent instance from the pinned PostgreSQL image and uses a temporary filesystem for its data. The host port is assigned by Docker on loopback only. Give the test login `CREATEDB` and a disposable password. Mount the password from a private file using `POSTGRES_PASSWORD_FILE`. Set `LGTM_TEST_POSTGRES` only for the command. The Server team's store tests, the existing C12 driver and the mutation hook inherit the variable. Remove the temporary directory and the uniquely named container on exit, including handled interruption. On an uncatchable stop, the fixture's outer supervisor or Docker operator removes the labeled test container and its private directory. Neither the password nor the connection string appears in command arguments, Docker configuration values or output. (architect-r2 3, threat-r2 5) The consumer needs no changes to its connection-string format. The production `postgres` service remains unpublished. **UNREVIEWED MECHANISM** (architect-r1 2). A command wrapper gives host tests the disposable database without adding a database service or printing the password in a connection string.

Stack writes `deploy/test/mutate-feed-lock.sh` for the existing driver to execute. It replaces itself with the executable `src/server/internal/store/check-feed-lock.sh`, which Server writes and owns. It inherits `LGTM_TEST_POSTGRES` and preserves the Server script's exit status. A missing or nonexecutable Server script fails with a named diagnostic. Document `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12` after the Server item merges. (architect-r2 3)

The setup and publication fixture proxies report status codes, named boundaries and request counts only. They write to per-test directories of mode `0700`. Use fixture-owned inputs and protected transient memory for any value comparison. Teardown removes the directories and the private password files. (threat-r2 5)

Freeze the following assertions in `deploy/test/vault-fixtures.sh`. A stub that reuses a project name or omits the database-creation privilege supplies a red test.

- `two_fixture_projects_clean_up` starts two projects as an ordinary Docker operator. It asserts distinct names and that teardown leaves neither project's resources, proxy report nor state. Check the private mode during the run and the report's field allowlist. The privileged helper stashes, fingerprints and restores a fixture key before teardown. (architect-r2 2, threat-r2 5)
- `test_database_is_disposable` invokes a host PostgreSQL client through the wrapper. It asserts authenticated database creation, child-only connection settings, loopback port publication, and cleanup when the child fails. It inspects Docker for the fixture port while the child runs. With a disposable sentinel password, check command arguments, rendered container configuration and output for absence of that value. Send a handled interrupt and check cleanup. (threat-r2 5)
- `mutation_hook_passes_through_server_status` exercises a disposable copy of the wrapper with an absent Server executable and then a fixture script. The second case verifies the inherited variable without logging it and checks zero and nonzero status passthrough. (architect-r2 3)

The item is risky because test setup handles credentials and creates isolated resources. Publish its port on every interface as the mutation. The network assertion must fail. Acceptance is `bash deploy/test/vault-fixtures.sh` reporting `ok` for all three assertions.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A development vault could appear to start while bypassing PostgreSQL. | High | Require PostgreSQL row growth and recover the original value after replacements. |
| A fixture could reuse an existing installation. | High | Use unique fixture names and remove only resources the fixture created. |

**Verify.** Run the proof and `bash deploy/test/vault-fixtures.sh` on Linux Docker. No device or Go server is needed.

**Rollout detail.** Step 1 releases later production work after the positive, key-control, root-generation and teardown observations pass.

**WS-2: Start PostgreSQL and open the vault inside the stack**

PostgreSQL stores application and vault data separately. The preparer checks ownership, database readiness and the existing seal before OpenBao restarts.

**Done when**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | Each service owns its database and is denied access to the other. A partial initialization resumes on the original volume. | `postgres_roles_are_isolated` and `postgres_repairs_half_initialized_volume`. |
| 2b | PostgreSQL stays unready until both role passwords authenticate. The live services use internal networks and publish no host ports. | `postgres_waits_for_both_databases` and `vault_has_no_host_port`. |
| 2c | The same seal key survives restart. A missing or wrong key with existing state cannot open the vault. | `seal_key_is_created_once`, `missing_key_preserves_existing_vault` and `wrong_key_preserves_existing_vault`. |

**Tasks**

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 2 | Create PostgreSQL and repair incomplete role setup. | `deploy/compose.yaml`, `services.postgres` and `volumes.pgdata`, modeled on `design/openpencil/compose.yml`. `deploy/postgres-init.sql` uses the pinned PostgreSQL image and PostgreSQL client anchors in scouting. |
| 3 | Prepare the key and start OpenBao. | `deploy/openbao.hcl`, `deploy/openbao-start.sh` and `deploy/stack-prepare.sh seal`. The OpenBao storage and static seal anchors are in scouting. |

**Work item 2. Create the PostgreSQL service and its local credentials**

Create `deploy/compose.yaml`, `deploy/.gitignore`, `deploy/postgres-init.sql`, `deploy/stack-prepare.sh`, `deploy/test/postgres.sh` and the C5 helper in existing `deploy/test/checks.sh`. Update the corresponding setting description in `deploy/test/README.md`. Stack writes these files. (architect-r2 1) Item 3 extends the Compose and prepare files after item 2 merges. Item 6 sequences the preparer commands. The database and seal suites exercise their production commands without the item 6 launcher. (logic-r1 9)

- Pin PostgreSQL to `postgres:16.16-alpine@sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d` for the daemon and the preparer. Item 1 confirms the digest on the build platform before merging. Use the named volume `pgdata` with `PGDATA=/var/lib/postgresql/data/pgdata`. Keep the volume's installation binding at `/var/lib/postgresql/data/lgtm-installation`, outside the PostgreSQL data directory. The preparer writes it and the launcher checks it before starting services. (logic2-r1 6, logic2-r1 8, threat-r1 6)
- The Compose project defaults to `lgtm`. The default state directory is `./.state` relative to the Compose file and is ignored by Git. `LGTM_STACK_STATE_DIR` overrides it with an absolute path. Production file binds use long syntax and `create_host_path: false`. The preparer refuses a directory at a file path and names the path and matching-state restore step. (architect-r1 10, critic-r1 10)
- `stack-prepare prepare-db` claims the installation using the binding protocol below. It makes one password at a time. It retains each valid existing password and fills in missing files only before `PG_VERSION` exists. A live cluster with absent or invalid credentials returns `restore the matching PostgreSQL password files`. File writes use a private temporary file and an atomic rename. Each password is an independent random 32-byte value encoded as 64 lowercase hexadecimal characters and one newline.
- Start `postgres` with `restart: unless-stopped` and `POSTGRES_PASSWORD_FILE`. Run the official entrypoint as user and group 70 with a prepared data directory. The image reads the protected file as its owner and skips the root user switch. (architect-r2 8, logic-r2 8) Set `POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 --auth-local=scram-sha-256` and `POSTGRES_HOST_AUTH_METHOD=scram-sha-256`. Salted Challenge Response Authentication Mechanism (SCRAM) protects loopback and socket connections as well as traffic from another container. Pass mounted password values to `psql` without putting them in arguments or output. (logic-r1 4, logic2-r1 3, threat-r1 4). **UNREVIEWED MECHANISM** (critic-r1 5). Enforcing passwords at initialization preserves the authenticated readiness requirement.
- `stack-prepare reconcile-db` runs as numeric user and group 70. It waits up to 60 seconds for an authenticated administrator connection to the final network listener. The launcher runs this command only after a volume claim is accepted. Through that connection, require every parsed row of `pg_hba_file_rules` to use `scram-sha-256` and have no parse error. If a rule is unsafe, print its method and line number without contents and stop before setup or OpenBao. This checks the file. The separate wrong-password probes check loaded behavior. Then run `postgres-init.sql` as a retryable setup script. (threat-r2 3) The image initializes the cluster. The reconciler owns application role and database creation, so retries do not depend on one-time image hook execution. Mark completed reconciliation as `pg.reconciled` in the state root after both network queries pass. The preparer writes it and the launcher reads it for diagnostics. Restore a corrupt cluster through the documented local restore path. (architect-r1 5, logic2-r1 4, critic-r1 3)
- Create missing logins with passwords from the retained files. A mismatched password on an existing login is a repair diagnostic rather than a silent rotation. Create `lgtm_app` with `OWNER lgtm_app` and `lgtm_bao` with `OWNER lgtm_bao`. These logins have neither `CREATEDB` nor `CREATEROLE`. The reconciler verifies the owners on existing databases. Revoke public connect on those databases, `postgres` and `template1`. Revoke public schema creation and leave each database owner able to create its own tables. OpenBao creates its storage table without an extended grant or a precreated-table setting. (architect-r1 8, logic-r1 1, critic-r1 1, threat-r1 5)
- Health executes authenticated `SELECT 1` over loopback for both role files. Use a five-second interval, a three-second timeout, 10 retries and a five-second start period for both services. A role setup wait or readiness failure exits with a phase-specific message.
- Create `data` and `vault` with `internal: true`. The database joins `data`. It publishes no port. Set `cap_drop: [ALL]` and `no-new-privileges:true` for each sprint 1 service. Apply the exact service and job sets below. No network-facing daemon or database reconciliation job gets a file-permission override. Use the first clean run to check the declared and observed process identities and sets. (architect-r2 8, logic-r2 8, threat-r2 8) The later server image must omit raw-network capability. (threat-r1 7)

| State file | Numeric owner and group | Mode | Writer and reader |
|---|---|---|---|
| `postgres-admin.password` | 70 and 70 | `0400` | `prepare-db` writes. PostgreSQL and the reconciler read. |
| `postgres-app.password` | 70 and 10001 | `0440` | `prepare-db` writes. The reconciler, readiness probe and later server read. |
| `postgres-bao.password` | 70 and 10002 | `0440` | `prepare-db` writes. The reconciler, readiness probe and OpenBao read. |

Mount PostgreSQL's three files by basename at `/run/lgtm-db/`. Use the same files for authenticated reconciliation and readiness. Keep the health probe's passwords in the subprocess rather than container settings.

**Installation binding and private directories** (architect-r2 2, logic-r2 9)

Item 2 writes the binding and directory functions in `deploy/stack-prepare.sh`. Item 3 uses the private directory layout. Items 6 and 7 reuse the binding check and run under the state-root directory lock specified in item 6. Item 2 tests the contract directly. **UNREVIEWED MECHANISM** (architect-r2 2). Put the production contract in the item that creates its script and the fixture helper in item 8. Both can merge before the seal and launcher items, within the epic's nine-item limit.

- The invoking operator owns the state root at mode `0700`. The offline preparer makes `bao/`, `admin/` and `server/` root-owned at mode `0700`. For a fresh, claimed volume it makes `PGDATA` owner and group 70, mode `0700`, under a volume root the daemon can traverse.
- The preparer writes local `installation.id` with version 1, project, a randomly generated installation identifier and a phase of provisional or committed. Keep it readable by the state-root owner only. The paired volume file `lgtm-installation` holds version 1, project and the same identifier. The preparer writes it by atomic hard link from a temporary file named `lgtm-installation.<installation-id>.tmp` on the same volume. It commits the local record only after the link is durable. The launcher, reset and maintenance check the pair. A matching provisional identity can commit on retry. Only the project and installation identifier determine ownership. Derive the canonical path for diagnostics on each run. This permits moving an intact state directory or reaching it through a symlink to the same directory inode. **UNREVIEWED MECHANISM** (logic-r2 3). Stable identity removes the need to rewrite ownership after a checkout move.
- A losing claim removes only the provisional identity it created or resumed. The claim occurs before making password files or private directories. Decline an unclaimed volume containing PostgreSQL data. Refuse a claimed volume with no local identity and refuse a committed local identity with no volume. Report these cases to the launcher before it changes service mounts. If two roots disagree on an identifier, the losing root emits `Compose project belongs to another state directory`. Use a different project with a fresh state root, or point at the matching state root for the existing project.

**Container users and capability limits** (architect-r2 8, logic-r2 8, threat-r2 8)

The Stack writer declares these users and capabilities. The runtime inspection in items 2, 3, 5b, 6 and 7 reads the rows for the processes those items add.

| Process | User and group | Added capabilities |
|---|---|---|
| `postgres` and its health and database check subprocesses | 70 and 70 | None. |
| Database reconciliation and the seal database probe | 70 and 70 | None. |
| `openbao` and its health command | 10002 and 10002 | None. |
| Offline private-file preparer and reset helper | 0 and 0 | `CHOWN`, `DAC_OVERRIDE` and `FOWNER` to handle protected files, volume ownership and retained state. They join no network. |
| `openbao-init` and the local maintenance tools job | 0 and 0 | `CHOWN` and `DAC_OVERRIDE` to publish for user 10001 and validate the protected final file. They join only `vault`. |

Freeze these tests in `deploy/test/postgres.sh`. Use a fixture Compose override with missing grants, a trusted loopback rule or a disposable volume that is not retained to reach the red assertions.

- `postgres_roles_are_isolated` connects from another test container. It checks database ownership, scratch writes, denied cross-database reads including the vault's storage table, and denied role or database creation. It runs OpenBao using the disposable proof setup and requires the storage table to appear under the vault login.
- `postgres_waits_for_both_databases` starts the database daemon before invoking reconciliation and checks the unhealthy state. After reconciliation it independently changes each mounted role password while leaving the logins enabled. Each substitution makes both the health command and the affected network login fail. Restore the original password between the two controls and verify health. (logic-r2 7) No sleep or hook inside the production initialization script is necessary. (critic-r1 2)
- `postgres_has_no_trust_rules` checks fresh-file rules and both role controls. In a separate claimed fixture initialized without the required flags, reconciliation refuses a `trust` rule before it creates or modifies an application role. (threat-r2 3)
- `postgres_installation_binding_precedes_service_start` directly exercises the preparer before the launcher exists. It claims an empty volume, retries a matching provisional identity, then refuses a second root and removes only that root's provisional record. A populated unclaimed volume is preserved. Inspect the directory modes and the database and reconciliation identities and capability sets. (architect-r2 2, logic-r2 9)
- `postgres_check_driver_authenticates` invokes the exact shared `psql_in_stack` command with a query on standard input. Require a result from `lgtm_app` as `lgtm_app` and the affected login to fail with a wrong fixture file. Stack arranges for its test to source the driver functions without running the checks. Existing executable dispatch still runs the selected checks. (architect-r2 1)
- `postgres_repairs_half_initialized_volume` starts a fixture with the original password files and only `lgtm_app` created. It writes a sample row, replaces the container with the same volume, and runs reconciliation. Assert creation of `lgtm_bao`, retention of the sample and password files, and a no-op second reconciliation.

The item is risky. Removing the initialization authentication flags must fail the wrong-password or rule assertion. Granting public connection to the vault database must fail the access assertion. Skipping reconciliation must fail the partial-volume test. Probing only one role must fail the other role control. Acceptance is `bash deploy/test/postgres.sh` with six passing assertions. (logic-r2 7)

**Work item 3. Prepare the seal key and start OpenBao**

Create `deploy/openbao.hcl`, `deploy/openbao-start.sh` and `deploy/test/seal.sh`. Extend `deploy/compose.yaml` and `deploy/stack-prepare.sh`. Mount the configuration read-only at `/openbao/config/lgtm.hcl`. Use `openbao/openbao:2.7.3@sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9`, subject to item 1's confirmation. (architect-r1 7, logic-r1 7, logic-r1 8)

- `stack-prepare seal` authenticates to `lgtm_bao` as its owner. It checks `to_regclass('public.openbao_kv_store')` and, if present, checks for rows. On any query error it stops before writing a key. A row, bootstrap journal, or published credential is sufficient to refuse creation. The missing-key diagnostic is `seal key missing for existing vault`. The preparer uses the private directories created by item 2. Run the database query as user and group 70 before invoking the offline seal file operation. (architect-r2 2, logic-r2 9) (logic-r1 6)
- `bao/unseal.key` is a single regular 32-byte file with owner and group 10002 and mode `0400`. Mount the file read-only at `/run/lgtm-bao/unseal.key`. OpenBao reads it as user and group 10002 under a `seal "static"` block with key identifier `lgtm-1`. The key preparer preserves a regular, nonsymlink 32-byte file with those owners and mode byte-for-byte. Use this table after a successful database probe. (logic-r2 5)

| Key | Existing rows or local bootstrap evidence | Action |
|---|---|---|
| Absent | None | Create and flush one key. |
| Absent | Present | Print `seal key missing for existing vault`. |
| Malformed, including wrong mode | Either | Print `seal key invalid`, preserve the file and direct the operator to restore the matching key. |
| Valid | Either | Preserve every byte. |

A well-formed file with different bytes passes file validation. The active-health and data checks detect that difference.
- Set Compose `entrypoint` to `/bin/sh /openbao-start.sh` and `command` to `server`. Stack writes the wrapper and OpenBao consumes its arguments. The wrapper reads only the mounted vault database password, builds `BAO_PG_CONNECTION_URL` with `sslmode=disable`, and executes `bao server -config=/openbao/config/lgtm.hcl`. The explicit wrapper bypasses the image's development default and does not need the memory-lock capability or the unsupported `disable_mlock` setting. Use `storage "postgresql"` with `max_connect_retries=0`. (logic-r1 7, critic-r1 11)
- Use a Transmission Control Protocol (TCP) listener on the container interfaces at port 8200. Transport Layer Security (TLS) is disabled on these internal networks. OpenBao joins `data` and `vault`, publishes no port, and declares a dependency on healthy `postgres`. Its read-only credential mounts contain only the seal key and its database password. Set `BAO_ADDR` to loopback inside the service so `bao status -format=json` works for `vault_ready` in `deploy/test/checks.sh`.
- Report Compose health using initialized, unsealed and PostgreSQL storage status. The launcher distinguishes observable decryption failure and authentication refusal using fixed classifications. It never streams raw logs containing a connection string. If the cause is unavailable, print `OpenBao did not become active` with the failed phase. (logic-r1 6)

Freeze these tests in `deploy/test/seal.sh`. A preparer stub that leaves the key absent or accepts a published host port has a failing assertion.

- `seal_key_is_created_once` compares the original file's type, length, mode, ownership and fingerprint across preparation and restart. The privileged fixture helper performs the file inspection.
- `missing_key_preserves_existing_vault` seeds a fixture record, stops the vault, and stashes the key through the fixture helper. Assert the named refusal, no replacement key and preserved rows. Restore the key and read the original value.
- `wrong_key_preserves_existing_vault` installs a different 32-byte file with the same mode. Assert no healthy or unsealed state and an unchanged database. Reinstall the original file and read the record.
- `seal_refuses_with_journal_and_empty_database` and `seal_refuses_with_credential_and_empty_database` seed only the named local evidence. Each asserts that no key is generated and setup returns the missing-key diagnostic. (logic-r1 6)
- `seal_refuses_malformed_file` repeats with zero length, wrong length, wrong mode and a symlink. Require a named refusal and unchanged records and bytes. (logic-r2 5)
- `openbao_runs_with_postgres_storage` requires `storage_type=postgresql` and checks the owner-created table.
- `vault_has_no_host_port` and `stack_services_have_no_egress_and_no_net_raw` inspect runtime port bindings, the declared internal networks and capability sets. Use an isolated control endpoint and test-owned probe on each network. Require no external route and exact identity and capability sets for the database, reconciliation job, offline helper and OpenBao. Run the gateway scenario in PB9 separately. An internal bridge can reach host listeners bound to its bridge address or to every interface. Keep sensitive host listeners off those addresses. Stack documents the limit in `deploy/README.md`. (architect-r2 8, logic-r2 8, threat-r2 4, threat-r2 8)

The item is risky. Mutate the guard to check database rows alone. The journal and credential controls must fail. Use the default development command as a second mutation. The storage-type test must fail. Acceptance is `bash deploy/test/seal.sh` with the nine named results passing.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A partial database setup leaves one role unavailable on retry. | High | Reconcile before gating on health and exercise the partial-volume fixture. |
| An existing vault is paired with a different key or state root. | High | Verify the installation binding and check existing storage before a write. |
| A container starts with an unintended network or privilege. | High | Inspect runtime networks, ports and capabilities on the pinned images. |

**Verify.** Run `bash deploy/test/postgres.sh` and `bash deploy/test/seal.sh` on Linux Docker against real PostgreSQL and OpenBao. No device or Go server is needed.

**Rollout detail.** Step 2 calls `prepare-db`, starts the database, calls `reconcile-db`, waits for health, and then calls `seal` before starting OpenBao.

**WS-3: Hand off a restricted server credential**

Bootstrap prepares a local maintenance identity and a separate server identity. It records the point at which it has enough material to resume and verifies that first-start privileges are revoked before handing the server its file.

**Done when**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 3a | The server file obtains a 15-minute token for the signing-key path. It is denied at sibling, tool and administrative paths. | `app_role_access_is_scoped` and `app_role_can_log_in_again`. |
| 3b | Root revocation precedes publication, and a crash before or after publication can resume from the durable checkpoint. A completed start validates the file without changing the policy or role. | `bootstrap_revokes_root_before_publish`, `bootstrap_resumes_at_publication_boundary` and `bootstrap_is_idempotent`. |
| 3c | A stack restart reopens the vault. A mismatched root or volume stops before changing the working installation. | `stack_reopens_existing_vault` and `stack_refuses_foreign_state_root`. |
| 3d | The local operator can replace the server credential after root revocation. The replacement can read the original signing key, and the old identifier cannot log in. | `admin_can_rotate_and_widen_policy_after_root_revocation`. |

**Tasks**

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 4 | Configure the server policy and both AppRoles. | `deploy/openbao-provision.sh`, the `lgtm-server` and `lgtm-admin` roles. Scouting links the upstream AppRole and policy contracts. |
| 5a | Initialize OpenBao and save recoverable material. | `deploy/openbao-bootstrap.sh`, `PUT /v1/sys/init`. The planned `deploy/openbao-init.sh` entry is in `floorplan/plan.json`. |
| 5b | Revoke initial root access and hand off the credential. | `deploy/openbao-init.sh`, `bootstrap.state`, and `server/openbao.json`. Scouting links the storage and rename contracts. |
| 6 | Launch in order and protect installation state. | `deploy/up.sh`, the command expected by `deploy/test/checks.sh`, `check_c1` and `vault_ready`. |
| 7 | Give the operator a local maintenance command. | `deploy/vault-admin.sh`, `apply-server-policy` and `rotate-server-credential`. The nearest contract is OpenBao's AppRole and policy endpoints in scouting. |

**Work item 4. Provision the OpenBao identities**

Create `deploy/openbao-tools/Dockerfile`, `deploy/openbao-tools/.dockerignore`, `deploy/.dockerignore`, `deploy/openbao-server-policy.hcl`, `deploy/openbao-admin-policy.hcl`, `deploy/openbao-provision.sh` and `deploy/test/vault-policy.sh`. Build only from the isolated `./openbao-tools` context. The Dockerfile has no project file copy. The context allows only the Dockerfile and ignore file. The outer ignore also excludes `.state`. Pin the helper's base to item 3's digest and pin every installed package using the full Alpine version. This includes `curl`, `jq` and GNU `coreutils` with `mv --no-copy` support. Build only when the helper for the reviewed Dockerfile and base is missing. Record its image identifier and base digest in the proof receipt. If an exact package version is unavailable, stop and name the missing pin. (threat-r2 7, logic-r2 1) (architect-r1 7, critic-r1 10, logic-r1 8, threat-r1 6, threat-r1 8, logic2-r1 8)

- Enable the version 2 key-value engine at mount `lgtm` and enable AppRole at `auth/approle`. Create role and policy `lgtm-server`. Bind a reusable secret identifier using `secret_id_ttl=0` and `secret_id_num_uses=0`. Set `token_type=batch`, `token_ttl=15m`, `token_max_ttl=15m` and `token_no_default_policy=true`. Grant read, create and update at `lgtm/data/server/link/signing-key`. Another server path needs a policy change through item 7. (architect-r1 4)
- Create role and policy `lgtm-admin`. Set `secret_id_ttl=0` and `secret_id_num_uses=0`. Its identifier remains usable until reset. This sprint supports no separate operator-identifier rotation. Losing the identity requires the matching-state restore or explicit reset procedure. AppRole login issues nonrenewable 15-minute batch tokens with no default policy. (threat-r2 6)
- The operator token has direct management access to the one named server policy and server identifiers. Grant read and update on `sys/policies/acl/lgtm-server`. Under `auth/approle/role/lgtm-server/`, grant read on `role-id`, list on `secret-id`, and update on `secret-id/lookup`, `custom-secret-id`, `secret-id-accessor/lookup` and `secret-id-accessor/destroy`. The setup script writes the protected `admin/operator.json` before `provisioned`. The maintenance job reads it. **UNREVIEWED MECHANISM** (threat-r1 1, logic2-r1 2).
- Treat the file as standing vault administrator material. Its holder can widen the server policy and obtain a server identity with the resulting policy. This means effective read or modification access anywhere the server can be authorized, including every stored secret. The initial server policy is still exact-path. The operator token itself has no direct data or root-generation access. Document this boundary in `deploy/README.md`. (architect-r2 7, threat-r2 1)
- The provisioner reads `admin/root.token` and stages both candidate identifiers before registration. A lookup with a matching accessor means present. The frozen test establishes the exact known absent-identifier result on the pinned image. Stop on storage, authorization and transport failures. Validate both logins and make `admin/operator.json` durable before advancing. The file table in item 5b specifies the writers and readers. Use protected streams or files for request values. **UNREVIEWED MECHANISM** (threat-r1 9). Measuring the negative response prevents an unrelated error from becoming permission to register a candidate.
- Completed bootstrap preserves its installed policy. Item 7 applies explicit policy updates. The initial exact-path policy does not authorize the server to discover other vault paths.

Freeze tests in `deploy/test/vault-policy.sh`. Use the shared proof fixture without waiting for work item 2. An empty fixture policy makes the positive access assertion fail.

- `app_role_access_is_scoped` verifies a nonrenewable 900-second lease, create/read/update on the signing-key path, and denial of sibling reads and writes, tool reads and writes, listing metadata, policy changes and token creation. An attempted second compare-and-set create must conflict.
- `app_role_can_log_in_again` verifies two distinct logins from the same file, denial for a wrong identifier, and reuse of the same accessor after provisioner replay.
- `operator_token_has_no_direct_data_access` probes management of the intended policy and identifiers, and denial of direct reads, direct root generation and unrelated mount operations. Inspect the protected file's mode and mounts. The disposable fixture separately grants the server one extra test-only secret path using the authorized maintenance operation, verifies the server can read it, and restores the original policy. This documents the operator's indirect authority. (architect-r2 7, threat-r2 1)
- `secret_id_lookup_classifies_absence` records the pinned response for an unregistered candidate. Verify a valid lookup reuses the accessor. Inject a revoked-token response and a transport failure and verify neither triggers registration.
- `images_are_pinned_by_digest` checks the image references and the helper base against the proof digests once Compose is merged. The item-4 check covers its Dockerfile. The item-6 merge check covers the rendered stack.
- `helper_packages_are_pinned` rejects any installed package without an exact version. A clean build records the image identifier and verifies rename-only support. (threat-r2 7, logic-r2 1)
- `helper_context_excludes_state` builds a disposable helper with sentinel fixture material under `.state`. The build input and layers must exclude that material.

The item is risky. Broaden the server policy and expect the sibling test to fail. Set the runtime identifier to one use and expect the second-login test to fail. Acceptance is `bash deploy/test/vault-policy.sh` with seven passing assertions.

**Work item 5a. Initialize OpenBao and save bootstrap material**

Create `deploy/openbao-bootstrap.sh` and `deploy/test/vault-bootstrap.sh`. This script owns the initialization checkpoints. Item 5b invokes it in the setup container and consumes the material it produces. Split the two scripts so each builder has its own frozen assertions. (critic-r1 7)

Use one versioned local checkpoint file `admin/bootstrap.state`, mode `0600`, owned by root. Stack's bootstrap and publication scripts write it by atomic replacement. The provisioner and publication job read it, and the prepare helper, launcher and maintenance job inspect it. Store version 1 and a named step. Refuse unknown versions, unknown steps and truncated files with `vault bootstrap checkpoint is invalid`. Its step is one of `intent`, `initialized`, `provisioned`, `revoked` or `published`. A stage advances only after its inputs and outputs are durable. Each private file and its containing directory are flushed before advancing. (logic-r1 2, threat-r1 9)

Query initialization with `GET /v1/sys/init`. Before the initial request record the `intent` checkpoint. Send `PUT /v1/sys/init` with `recovery_shares=1` and `recovery_threshold=1`. Save the response directly to `admin/init-response.json` and flush before parsing. A complete response is a regular private file containing one valid object with a nonempty string at `recovery_keys_base64[0]` and a nonempty string at `root_token`. Save those two values to `admin/recovery.key` and `admin/root.token`. A trusted root token is one that passes a valid token lookup. The provisioned row below also handles a token whose recorded revocation now returns 403. (logic-r2 5) Both have mode `0600`. Advance to `initialized` and then remove the response JSON. If the response has been lost after initialization, setup follows the state table in item 5b. (threat-r1 2)

Treat initialization and readiness separately. Poll `GET /v1/sys/health` for status 200 before provisioning or validation, including after a successful init. Start one 60-second deadline when the setup job starts. Read elapsed time from the Linux monotonic boot-time clock exposed by `/proc/uptime`. Status 503 means wait only until that deadline and bound each request by the time left. If the vault fails to become active in time, print `OpenBao did not become active` and exit nonzero as a retryable wait failure. Preserve the current checkpoint and files. The later Compose wait is independent. (logic-r2 11) (logic2-r1 7)

Freeze assertions in `deploy/test/vault-bootstrap.sh`. A stub that advances without writing durable material makes the file and resume assertions fail. The test runs a test-owned recording proxy on `vault` and points only the fixture's `BAO_ADDR` at it. The proxy forwards to real OpenBao. It can hold a response before delivery and signal via the private status-only report from item 8. (threat-r2 5) The test kills the fixture job through Docker when the proxy holds the selected response. For local write boundaries, a fixture-only command shim reports the response-file and checkpoint renames. The test kills the fixture job after the named rename. (critic-r1 2)

- `bootstrap_saves_material_before_continuing` interrupts setup with a saved response and then at the initialized checkpoint. It verifies root and recovery material survive before the next request and that the second run performs no extra init.
- `bootstrap_refuses_incomplete_state` lets the server commit initialization, drops the response, and kills the job. The second run must emit `vault is initialized but bootstrap state is incomplete` and keep the records. The launcher tests exercise the guarded operator reset.
- `bootstrap_waits_for_active_vault` holds health at 503 before allowing the 200 response. Assert that no provisioning request goes through before 200. In a second case keep 503 through the deadline and assert the wait diagnostic, no state loss and a successful retry. (logic-r2 11)
- `bootstrap_refuses_invalid_local_material` uses separately truncated response and checkpoint files and an unknown checkpoint version. Require the matching diagnostic, no second init and preserved records. Repeat with an uninitialized fixture and stale checkpoint or response. Require the state-ahead diagnostic without the item 6 launcher. (architect-r2 2, logic-r2 5)

The item is risky. Move the checkpoint update ahead of the material write and expect the first assertion to fail. Treat initialization as readiness and expect the third assertion to fail. Acceptance is `bash deploy/test/vault-bootstrap.sh` with four passing assertions.

**Work item 5b. Publish the credential and verify completed startup**

Create `deploy/openbao-init.sh` and `deploy/test/vault-init.sh`. Add the `openbao-init` one-shot service under Compose profile `setup`. It uses the helper image and joins `vault`. Set its `BAO_ADDR` from service `openbao` and port 8200. Mount `admin/` and `server/` separately at `/state/admin` and `/state/server`. Mount both initialization scripts, the provisioner and the policies read-only. Normal setup has no seal key or database password mount. Inspect the job user and effective capability set against item 2 in the frozen test. (architect-r2 8, threat-r2 8)

The following table is evaluated in order. A conflicting binding stops the launcher before it runs this job. A failed probe stops the job before it changes state. (logic-r1 2, architect-r1 6, threat-r1 3, threat-r1 9)

| OpenBao and local state | Outcome and observation |
|---|---|
| Checkpoint invalid or version unknown | Print `vault bootstrap checkpoint is invalid`. Preserve evidence. `bootstrap_refuses_invalid_local_material`. |
| Uninitialized with no committed material. A missing checkpoint or bare `intent` is allowed. | Initialize once. Resume from the successful response. `bootstrap_saves_material_before_continuing`. |
| Uninitialized with a later checkpoint, a saved response, another bootstrap object or a published file | Print `state directory is ahead of the database` and preserve both stores. `bootstrap_refuses_invalid_local_material`. |
| Initialized with absent or `intent` checkpoint and a complete private response. Final credential absent. | Save and check the response. Advance to `initialized`. `bootstrap_saves_material_before_continuing`. |
| Initialized with absent or `intent` checkpoint and an absent or incomplete response | Print `vault is initialized but bootstrap state is incomplete`. Preserve both stores. `bootstrap_refuses_incomplete_state` and `bootstrap_refuses_invalid_local_material`. |
| An initialized vault fails to become active by the deadline | Print `OpenBao did not become active` and leave files and checkpoint intact for retry. `bootstrap_waits_for_active_vault`. |
| Active at `initialized`. Valid saved root and recovery material exist. Final credential absent. | Provision or resume the two staged identities. Verify both logins. Save `provisioned`. `bootstrap_resumes_staged_registration`. |
| Active at `provisioned`. The saved root token file and verified durable operator and pending server credentials exist. | Remove the now-unused recovery file. Revoke the initial token using `POST /v1/auth/token/revoke-self`. Confirm 403 on its lookup and revalidate the two staged identities on retry. Persist `revoked` before deleting the token file. `bootstrap_revokes_root_before_publish`. |
| Active at `revoked`. Durable operator and pending server credentials exist, and the final file is absent. | Revalidate the objects and publish. Advance to `published`. `bootstrap_resumes_at_publication_boundary`. |
| Active at `revoked` with a valid final file and durable operator file, or at `published` with both files valid | Validate the final file and authenticate once using the server credential. Advance the checkpoint if needed. Remove only bootstrap's staging files. Print `vault already configured` on a completed rerun. `bootstrap_is_idempotent`. |
| Active at `revoked` with no usable pending or final server file | Print `vault bootstrap publication material needs restore`. The operator file was durable before revocation. Keep that file and the volume. Restore the matching staged file or use the guarded installation reset if no usable backup exists. `bootstrap_refuses_invalid_published_credential`. |
| Active at `published` with missing, malformed or rejected server credentials and a valid operator file | Print `server vault credential needs repair`. The operator can run item 7's rotation. Preserve the vault and operator credential. `bootstrap_refuses_invalid_published_credential`. |
| Any other combination or missing required material | Print `vault is initialized but bootstrap state is incomplete` and preserve evidence for local recovery. `bootstrap_refuses_invalid_published_credential`. |

The operator file at `published` is required for maintenance. Completed startup validates its shape and mode without an extra operator login. It requires neither the initial token nor the recovery share. **UNREVIEWED MECHANISM** (logic-r2 6). The recovery share cannot complete any supported operator action. Keeping it only until `provisioned` makes its removal and the checkpoint requirements compatible. (threat-r2 6, threat-r2 9)

| Protected file | Writer and reader | Lifetime |
|---|---|---|
| `admin/bootstrap.state` | Bootstrap and publication write it. Prepare, provision, publication, launch and maintenance read it. | Retain through reset. |
| `admin/init-response.json` | Bootstrap saves and validates it. The publication dispatch recognizes its presence. | Delete after `initialized` is durable. Bootstrap cleanup may clear a leftover after validating that checkpoint. |
| `admin/root.token` and `admin/recovery.key` | Bootstrap writes them. Bootstrap checks both at `initialized`. Provision and publication read the token. | Delete the share after durable `provisioned`. Delete the token after durable `revoked`. The next matching checkpoint can finish interrupted cleanup. |
| `admin/bootstrap-server.pending.json` and `admin/bootstrap-operator.pending.json` | Provision writes each staged credential before registration and uses the same value on retry. Publication reads the pending server credential. | Remove the pending operator file after the durable operator file. Remove the pending server file only after durable `published`. |
| `admin/operator.json` | Provision writes exactly the two nonempty string fields `role_id` and `secret_id`, owned by root with mode `0600`. Publication verifies it. Maintenance reads it. | Keep through reset. |
| `server/.openbao.bootstrap.pending.json` | Publication copies and flushes the verified candidate with final owner and mode. It reads or clears this exact staged name during bootstrap recovery. | Rename during publication. Only bootstrap clears leftovers under this name. |
| `server/openbao.json` | Publication and maintenance write it. Bootstrap validates it. The sprint 2 Server reads it. | Replace by controlled rotation. Remove on a verified reset. |
| `admin/rotation.state` and `server/.openbao.rotation.pending.json` | Maintenance owns both. Startup reads only the presence of the journal. The staged credential and accessor record have separate lifetimes described in item 7. | Keep until the rotation finishes successfully. Bootstrap cleanup never removes either name. |

Stack uses these names under the private directories, with the two bindings at `/state/admin` and `/state/server` in the tools container. Each producer flushes its private temporary file and directory before advancing. The reset helper reads the list to check ownership of the files it is allowed to remove. (logic-r2 4, logic-r2 5, threat-r2 9)

Publish using GNU `mv --no-copy -T` from `/state/server/.openbao.bootstrap.pending.json` to `/state/server/openbao.json`. Flush the destination directory before advancing. The pending server credential survives until both publication and its checkpoint are durable. **UNREVIEWED MECHANISM** (logic-r2 1). The production command fails if the kernel refuses the rename and cannot fall back to copying. Retain the original within-mount requirement. (logic2-r1 1, architect-r1 3). **UNREVIEWED MECHANISM** (logic2-r1 5). Retention plus one-login validation rejects a published file that no longer matches restored storage.

**Contract for sprint 2** (architect-r1 9)

| Surface | Stack writes or supplies | Sprint 2 reads or extends |
|---|---|---|
| Container user | The Stack team sets numeric user and group 10001 in the server image. | The server process uses the mounted credential files as that user. |
| Compose names | Stack publishes no host port for `postgres` or `openbao`. PostgreSQL uses `data`, and OpenBao uses `data` and `vault`. | Stack joins the server to both internal networks. The web proxy stays on its own server-facing network. |
| Database | Stack mounts `postgres-app.password` read-only at `/run/lgtm/postgres.password`, retaining owner 70, group 10001 and mode `0440`. Stack sets `LGTM_POSTGRES_PASSWORD_FILE` to that path, `LGTM_POSTGRES_HOST=postgres`, `LGTM_POSTGRES_PORT=5432`, `LGTM_POSTGRES_DATABASE=lgtm_app` and `LGTM_POSTGRES_USER=lgtm_app`. | The Server settings read these five names and consume the file as one hexadecimal password followed by a newline. (architect-r2 5) |
| Check driver access to the database | In item 2 Stack updates `psql_in_stack` in `deploy/test/checks.sh`. Use `compose exec -T --user 70:70 postgres`. Inside it, read `/run/lgtm-db/postgres-app.password` into the `psql` subprocess environment, use `psql -w -h 127.0.0.1 -U lgtm_app -d lgtm_app`, and keep the existing options, bound arguments and standard input. | C5 consumes this command in sprint 3. The item 2 suite exercises the same helper now and the password stays out of arguments and output. (architect-r2 1) |
| Vault credential | The initializer writes default `deploy/.state/server/openbao.json`, or `server/openbao.json` under `LGTM_STACK_STATE_DIR`. It is 8-bit Unicode Transformation Format (UTF-8) JavaScript Object Notation (JSON) with exactly the two nonempty string fields `role_id` and `secret_id` and one trailing newline. Owner and group are both 10001, mode `0400`. Stack mounts it read-only at `/run/lgtm/openbao.json`. | The Server team reads `LGTM_OPENBAO_CREDENTIAL_FILE` and `LGTM_OPENBAO_ADDR`. Stack sets them to the mount path and the HTTP service `openbao` on port 8200. The server signs in at `POST /v1/auth/approle/login`, uses `auth.client_token` in `X-Vault-Token`, and reauthenticates before its lease expires. |
| Signing key | Stack grants access to `lgtm/data/server/link/signing-key` in OpenBao. | The Server team writes and reads `private_seed_b64`, a standard base64-encoded 32-byte Ed25519 seed. The first write uses `options.cas=0`. A creation conflict leads to a read of the stored key. |
| New vault path | Stack edits `deploy/openbao-server-policy.hcl` after the Server team supplies its path and operations. The local operator reviews the diff and runs `deploy/vault-admin.sh apply-server-policy --yes`. | The later Server plan uses the agreed path after the operator applies the policy. No runtime startup edits policy. (architect-r2 5, threat-r2 1) |
| Signing-key load event | Server emits one structured log object with `msg` equal to `link_signing_key_loaded`, `source` equal to `openbao` and a nonempty public string `fingerprint`. | Stack's existing `server_key_fingerprint` in `deploy/test/checks.sh` reads those exact fields for C13. (threat-r2 10) |
| Launcher | Stack writes executable `deploy/up.sh` in sprint 1. | The sprint 2 Stack plan extends that file. It starts the server only after OpenBao bootstrap succeeds. An atomic credential replacement needs a new server container to bind the new inode. |

Freeze these tests in `deploy/test/vault-init.sh`. A stub that claims success without publishing a file reaches red assertions. In the disposable fixture the proxy emits only the safe boundary report. A fixture-only `mv` shim requires `--no-copy -T` for final publication. It signals before or after calling the real pinned command and forwards its exit status. The test kills the container at the named signal. The real fixture mounts `admin` and `server` separately. The successful publication must pass through this shim. (logic-r2 1, threat-r2 5) (critic-r1 2, architect-r1 3)

- `bootstrap_revokes_root_before_publish` asserts the captured initial token gets 403 immediately before the file appears. The positive file can log in and has the specified owner, group, mode and shape.
- `bootstrap_resumes_staged_registration` interrupts just before and just after identifier registration using the fixture proxy. A rerun registers one identifier per role and publishes once.
- `bootstrap_resumes_at_publication_boundary` runs the revoke and the before/after-rename interruptions in distinct fixtures. Each retry leaves one valid credential. Moving the candidate to the real admin mount must fail the unmodified positive path and leave the final path absent. Dropping the rename-only flag must fail the shim's required observation. A test mutation that copies to the final path directly must fail that same observation even if it later leaves a complete file. (logic-r2 1) Test observations include no captured credential in logs or container configuration.
- `bootstrap_is_idempotent` compares the static seal and server credential fingerprints and modification times. It verifies the setup identity and exact capability set and checks that the transient root token and recovery share are absent on a completed run. (logic-r2 6, threat-r2 6, threat-r2 8) The proxy observes one server login, no provisioning requests, and no change to state on a clean second run. After an interrupted finalization it observes cleanup without replaying provisioning.
- `bootstrap_refuses_invalid_published_credential` supplies missing, malformed and revoked server files at `published`, and missing pending and final files at `revoked`. It checks the distinct diagnostics and that the operator file stays intact. (threat-r2 9) It verifies the repair diagnostic and unchanged vault contents.

The item is risky. Remove root revocation, move the pending-file cleanup before publication, or use a cross-mount rename, one per mutation. Each must fail the corresponding assertion. Acceptance is `bash deploy/test/vault-init.sh` with five passing assertions.

**Work item 6. Start and verify the sprint 1 stack with one command**

Create executable `deploy/up.sh`, `deploy/README.md` and `deploy/test/stack-vault.sh`. Extend `deploy/stack-prepare.sh` for the bounded reset operation. Reuse item 2 for binding and private directory preparation and item 8 for fixture inspection. (architect-r2 2, logic-r2 9) The default invocation from the repository root is `deploy/up.sh`. The existing check script calls it directly. Use one Test Anything Protocol (TAP) line per named assertion in the retained suites, with `ok` or `not ok`, an ordinal and the test identifier. (critic-r1 11)

On the local Linux host, the launcher takes an exclusive `flock` on the canonical state-root directory inode before it preflights anything. Maintenance and reset take the same lock. Preserve that directory inode through reset. A plain startup never deletes retained files or volumes. The volume claim from item 2 still serializes two distinct state roots for one Compose project. A refused claim reaches no service start or mount change. A Docker-capable operator uses the documented helper to inspect permitted state without printing file contents. Restore of protected files requires that helper or administrator privileges. (critic-r1 4, logic2-r1 6, logic-r2 2)

Check for `reset.pending` before preparing the store. If present, print `reset in progress; run deploy/up.sh --reset --yes to finish` and exit nonzero. Check for `admin/rotation.state` before bootstrap cleanup or server recreation. If present, print `credential rotation is pending; run deploy/vault-admin.sh rotate-server-credential` and exit nonzero. The reset helper and maintenance job write those files. Startup reads them. (logic-r2 2, logic-r2 4)

Run these phases under the lock. Emit `phase <name> start` and `phase <name> done` at each phase boundary. The launcher writes these fixed names and the frozen fixture reads them. (critic-r1 2)

| Phase | Operation |
|---|---|
| `database` | Run `stack-prepare prepare-db`. Start `postgres` without waiting for health. Run `stack-prepare reconcile-db`, then wait for both role queries. |
| `seal` | Run `stack-prepare seal`. It refuses inconsistent storage or files before OpenBao starts. |
| `vault` | Start `openbao`. Build the pinned tools image only if the expected build for the reviewed inputs is absent. Run `openbao-init` and wait for both daemons with `docker compose -f deploy/compose.yaml up -d --wait --wait-timeout 60 postgres openbao`. |
| `ready` | After success, print `postgres and openbao are healthy`. A failed earlier phase exits with its specific diagnostic. |

Use `docker compose -f deploy/compose.yaml run --rm --no-deps <job>` for one-shot jobs. `deploy/up.sh` is the supported first-start command. Compose can recreate already initialized daemons without running setup. A plain Compose `down` preserves the named volume and state. For configured installations supply the same `COMPOSE_PROJECT_NAME` and `LGTM_STACK_STATE_DIR` for maintenance. (architect-r1 10)

**Operator recovery** (logic-r1 3, threat-r1 2, threat-r1 3, critic-r1 3, architect-r1 6)

| Diagnosis | Operator action in `deploy/README.md` |
|---|---|
| Ordinary service restart or Compose shutdown | Use `deploy/up.sh` to verify both services. The same data and key remain. |
| The volume and identity match but a password or seal key is unusable | Stop the stack. Restore the matching local file with its documented owner and mode. To discard the installation instead, use the confirmed reset. |
| The volume is absent and the committed local state remains | Startup prints `state directory is ahead of the database`. Restore the matching volume or run the confirmed reset against the committed local identity. Reset removes only that local state. |
| The claimed volume remains and the local root has no state | Startup prints `restore the matching installation state`. Restore the matching state or use the orphan-volume reset described below. |
| The path changed and both identities match | Rerun from the new path. The installation identifier still owns the volume. |
| The installation identifiers differ | Startup and reset decline. Use the matching original state root, or pick an unused Compose project with a fresh root and preserve the existing volume. |
| An unclaimed volume already contains PostgreSQL data | Launch and reset refuse. Recover a known matching installation or use a fresh project. Preserve the unclaimed volume. |
| Vault bootstrap is incomplete | Preserve the installation while diagnosing. Restore the matching backup. The separately confirmed reset discards the selected installation when recovery is unavailable. |
| The published server credential needs repair | With active storage and the durable operator credential, run the rotation command. |
| Reset stopped before finishing | Rerun `deploy/up.sh --reset --yes` for the same project and state root. The marker retains verified intent. |

The reset requires `deploy/up.sh --reset --yes` under the same lock as startup. In the normal case verify both records. For local-only state, verify a committed local record for the selected project and confirm there is no current volume or service for it. For a claimed volume with lost state, require an empty state root and the explicit invocation `deploy/up.sh --reset --yes --orphan-volume-id <installation-id>`. Verify the exact identifier from the volume binding and the selected project's Docker Compose project and `pgdata` volume labels. The guide names the read-only inspection for those public identifiers. Ordinary reset cannot remove an orphan volume without that extra match. No mode removes an unclaimed data volume or a volume with a different identifier. **UNREVIEWED MECHANISM** (architect-r2 4, logic-r2 3). Explicit orphan confirmation lets an operator discard service data after losing local files without treating a foreign claim as a match.

Before making any destructive change, preflight the local manifest. Permit only `installation.id`, `pg.reconciled`, the three item 2 password files, `bao/unseal.key`, the listed item 5b files under `admin/` and `server/`, and `reset.pending`. A writer's only private scratch pathname is the target basename followed by `.tmp` in its directory. The reset helper recognizes only those exact siblings. Refuse a foreign entry, symlink or nested mount and name its relative path. Delete only verified manifest names, then remove the empty private subdirectories. Never recurse over the selected root. There is no lock file in the manifest. (threat-r2 2)

The launcher writes and flushes `reset.pending` with format version 1, selected project, installation identifier and the verified reset mode before stopping service or deleting data. The same pathname is used on replay. Recheck any volume before touching it. A matching volume still present can be removed after the selected services stop. A foreign replacement stops reset without stopping its services. Once the selected volume is absent, the offline `stack-prepare reset-state` job mounts only the local state. It verifies the pending intent and every remaining manifest entry. Delete the marker last and flush the state directory. The marker authorizes continuation after the local identity is removed or the selected volume disappears. Print only the phase and the diagnostic. A local-only reset skips Docker removal. (logic-r2 2, threat-r2 2). **UNREVIEWED MECHANISM** (logic-r1 3, threat-r1 2, critic-r1 3). An explicit bounded reset avoids guessing whether unauthenticated storage is empty.

Stack writes the reset flags, diagnostics and versioned marker. The offline helper and launcher read the marker. The operator reads the diagnostics and invokes the matching documented command. The guide also warns that checkout cleanup can remove ignored state and describes the existing absolute local override.

Freeze tests in `deploy/test/stack-vault.sh`. A launcher stub that succeeds without running setup fails on readiness. A test-only override holds the first launcher inside the `seal` phase using an isolated file signal before the second attempts entry. Use the phase lines to record ordering.

- `stack_reopens_existing_vault` checks that the launcher is executable, starts both healthy services, persists a fixture signing key, runs ordinary Compose `down`, and verifies the same key after restart through the mounted credential as user 10001.
- `stack_refuses_missing_key` stashes the key through the fixture helper after persisting data. Assert the missing-key diagnostic, no replacement key and no healthy vault. Restore the original and read the record.
- `stack_bootstrap_is_serialized` overlaps two invocations using the same state root. It checks the phase boundaries and one retained identity.
- `stack_refuses_foreign_state_root` overlaps two invocations with distinct roots for one Compose project and also reruns the loser sequentially. Require the binding diagnostic and prove that the winner's existing record and published credential remain usable.
- `stack_refuses_state_ahead_of_database` removes the fixture volume while preserving local state. Startup reports the mismatch. Confirmed local-only reset removes only the manifest and permits a fresh start. (architect-r2 4)
- `stack_requires_explicit_orphan_volume_reset` keeps the volume while losing local state. Assert the restore diagnosis and refusal without the matching explicit identifier. The verified orphan reset removes only the selected fixture volume. (architect-r2 4)
- `stack_preserves_unclaimed_data_volume` runs launch and reset against a populated volume without a binding. Both must refuse, and the original fixture row remains. (architect-r2 4)
- `stack_accepts_moved_state` moves the root after plain Compose shutdown and starts through the new path and a symlink to it. Check that its identity, volume and secret are unchanged. For a mismatched identifier, the existing foreign-root test must also show reset refusal and a cleaned provisional identity. (logic-r2 3, architect-r2 4)
- `stack_reset_requires_confirmation` uses a disposable incomplete bootstrap fixture. Assert the command without `--yes` leaves both stores intact. Assert the confirmed command removes that installation and a fresh startup passes. Keep a second fixture running and verify it survives the reset. Inspect the reset helper's identity and exact capability set. (architect-r2 8, threat-r2 8)
- `stack_resumes_interrupted_reset` pauses after volume removal. Kill the launcher, then require the pending-reset diagnostic. Retry with the same inode and marker and require clean setup afterwards. In a separate interruption, remove the local identity before stopping and require the same retry result. (logic-r2 2, threat-r2 2)
- `stack_reset_refuses_foreign_entries` places a foreign file, a symlink at a listed path and a nested mount in separate fixtures. Require refusal before volume removal and preservation of each fixture and its foreign object. (threat-r2 2)

The item is risky. Remove the state lock, bypass the volume binding, or ignore a failed seal step. Expect the respective serialization, foreign-root, or missing-key test to fail. Acceptance is `bash deploy/test/stack-vault.sh` with eleven passing assertions.

**Work item 7. Maintain the server credential from the local stack**

Create `deploy/vault-admin.sh`, `deploy/openbao-admin.sh`, and `deploy/test/vault-admin.sh`. Extend `deploy/README.md` after item 6 merges. The host launcher uses the same state lock and project validation as startup. A one-shot tools container on `vault` reads the operator credential and writes the server file. Only the setup and maintenance jobs mount `admin/`. (threat-r1 1, logic2-r1 2, critic-r1 9)

- Both maintenance actions require active OpenBao, a matching installation and a valid `published` checkpoint. Earlier checkpoints return `vault is initialized but bootstrap state is incomplete` before any maintenance write. A pending reset blocks both actions. A pending rotation blocks policy application and is resumed only by rotation. (architect-r2 6, logic-r2 4)
- `deploy/vault-admin.sh apply-server-policy` displays the difference between live `lgtm-server` policy and the reviewed `deploy/openbao-server-policy.hcl`. Require `--yes` to apply a difference. It changes only the named policy. Stack writes the flag and the policy file. The operator reviews the diff and chooses whether to apply it. **UNREVIEWED MECHANISM** (architect-r1 4). The server's startup path never widens policy. (architect-r2 5, threat-r2 1)
- `deploy/vault-admin.sh rotate-server-credential` initializes a version 1 journal at `admin/rotation.state` before registering anything. Record the phase, the superseded accessors and the candidate accessor once known. Store no SecretID in the journal. The durable `server/.openbao.rotation.pending.json` holds the candidate credential. Keep its protected source until rename, and keep the journal through old-accessor retirement. Startup checks the journal's presence and leaves rotation files alone.
- Record the previous accessor using the file's SecretID lookup if possible. If the old file is missing or unusable, take the server role's existing accessor list before registration. Durably stage one candidate before registering it. On replay use the same candidate, and verify its accessor and login before publication. Accessor lookup distinguishes an existing entry from the measured absent response. Other errors stop for retry. Record the candidate accessor before the rename.
- Publish using the same rename-only command and directory flush as item 5b. If the staged file is absent on retry after a successful rename, validate the final file against the journaled accessor before doing anything else. Recreate the server container if it exists and verify its mount uses the new file. If this fails, leave the old identifier active and keep the journal. Destroy only the superseded accessors after the fresh bind, then remove and flush the journal. If interrupted during retirement, verify absence on retry and continue. Existing tokens can remain valid for the rest of their 15-minute lease. (logic-r2 4, logic-r2 10)
- Use the durable operator credential to perform maintenance after the initial administrator token is revoked. Treat the file as administrator material. No supported maintenance operation uses a retained recovery share. The sprint 1 test uses a disposable reader at user 10001 and never publishes credentials to test logs. (threat-r2 1, threat-r2 6)

Freeze tests in `deploy/test/vault-admin.sh`. A command stub that returns success without registering or publishing a new credential makes these assertions fail.

- `admin_can_rotate_and_widen_policy_after_root_revocation` bootstraps once and verifies the original root token is denied. It requires a visible diff and no mutation without `--yes`, then confirms a fixture policy that adds one server-owned path. Compare the tools job's user and effective capabilities to item 2. (architect-r2 8, threat-r2 1, threat-r2 8) The new file logs in, reads the original signing key and reaches the added path. Tool and administrative paths remain unavailable. The superseded identifier cannot log in. A test reader with a fresh bind mount reads the new inode.
- `admin_resumes_credential_rotation` interrupts separate fixtures after the initial journal, after registration, before rename and after publication before retirement. The last case refuses reader recreation through the fixture Docker wrapper. Ordinary startup reports pending rotation in each case and preserves the fixture files. Rerun rotation, require one registered candidate and no orphan accessors, verify the old identifier cannot log in, and read the unchanged record using the new reader mount. (logic-r2 4, logic-r2 10)
- `admin_requires_published_checkpoint` exercises both commands at `initialized`, `provisioned` and `revoked`. Require the incomplete-state diagnostic and no policy write, registration, publication or change to the marked checkpoint. (architect-r2 6)

The item is risky. Skip old-accessor revocation, skip reader recreation, or remove the journal check. Expect the corresponding assertion to fail. Acceptance is `bash deploy/test/vault-admin.sh` with all three assertions passing.

**Post-build checks**

These checks cover behavior that needs the production files or the sprint 2 stack. The round 1 and round 2 registers map the checks to their reports.

| Check | Scenario and expected result |
|---|---|
| PB1. Frozen production image | Start the production image as its documented user on a clean Linux Docker host. Require restricted PostgreSQL table creation, the documented mounts and capabilities, and no directory created at a file bind after a premature bare Compose start. |
| PB2. Key and PostgreSQL outages | Restart OpenBao with a wrong key and with PostgreSQL absent in separate runs. Wrong key keeps the vault unready with the named diagnostic. Healthy PostgreSQL returning allows the original key to reopen the same record. OpenBao's unsealed status alone does not prove storage is reachable. Also check the missing-file behavior after Docker restarts and the 60-second timeout on a slow fixture. |
| PB3. Secret handling | Capture rendered Compose, container inspection, console and logs with fixture credentials, including wrong database authentication. Scan for the actual fixture values and connection URL. Expect no match. Check a different numeric process user cannot read OpenBao's process environment. Inspect private ownership and mount boundaries. |
| PB4. Bootstrap interruptions | Stop fixture jobs at the checkpoint transitions and after the initial request commits. Resume to one published credential or the exact documented refusal. Run the guarded reset only on disposable state. Verify a replacement file needs a fresh server bind. |
| PB5. Isolation at first start | Check that only the intended setup job is placed on the newly created vault network before initialization finishes. A Docker administrator attaching a new container to the private networks is outside this boundary. In sprint 2, confirm that no server container starts until the bootstrap job succeeds. |
| PB6. Volume without state | Attempt to start a disposable copy of the vault database without the matching local key. Require refusal and no readable secret. |
| PB7. Host memory | On a host with swap, record Docker's swap limits for the unprivileged OpenBao process. Record the host-memory exposure separately from the database-at-rest test. |
| PB8. Full sprint 2 restart | Run `deploy/test/checks.sh --only C13` after the server and website are built. Require both automatic unseal and the same public signing-key fingerprint after recreation. Sprint 1 uses `deploy/test/stack-vault.sh`. |
| PB9. Internal bridge boundary | Place a harmless listener on the disposable Docker host's bridge interface and probe it from each attached sprint 1 service on each internal network. Record whether host access succeeds. Require no external route and no published service ports. Document the bridge reachability observed. (threat-r2 4) |
| PB10. Database rollout hooks | Run the C5 query helper against the built stack and require one row from `lgtm_app`. Once the Server store merges, run `deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12` and require the suite and feed-lock mutation to pass. (architect-r2 1, architect-r2 3) |
| PB11. Reset and relocation | Exercise the moved root, missing volume, orphan volume and interrupted reset scenarios on separate disposable installations. Expect retained data on relocation or refusal, and successful removal only after a verified reset. (architect-r2 4, logic-r2 2, logic-r2 3, threat-r2 2) |
| PB12. Bootstrap and rotation | Use the exact job image, separate binds and fixture test boundaries. A cross-mount publication must fail without writing the final path. Truncated local material keeps the vault data intact. A stopped credential replacement makes ordinary startup fail with the pending diagnostic until rotation completes. (logic-r2 1, logic-r2 4, logic-r2 5) |
| PB13. Images and process capabilities | Confirm both registry digests for the build platform. Build the pinned helper and record its image identifier. An unavailable pin fails clearly. Check effective capability sets for each running service and job against the exact item 2 table. File-permission override is limited to the offline helper, setup job and maintenance job. (architect-r2 8, logic-r2 8, threat-r2 7, threat-r2 8) |
| PB14. Operator authority | In a disposable fixture, establish direct denials and the approved policy-change effect on a fixture secret. Check direct root-generation denial with the operator token and with no token. Record the authority boundary and absence of a retained recovery share after production bootstrap. (architect-r2 7, threat-r2 1, threat-r2 6) |

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A partial bootstrap revokes privileged access before a runtime credential is usable. | High | Verify both identities and keep the pending file until durable publication. |
| The server role gains administrative or other-owner access. | High | Keep separate file mounts and require denied policy and data operations. |
| A reset targets the wrong installation. | High | Verify the binding, require an explicit confirmation argument and exercise a second unaffected fixture. |

**Verify.** Run `bash deploy/test/vault-policy.sh`, `bash deploy/test/vault-bootstrap.sh`, `bash deploy/test/vault-init.sh`, `bash deploy/test/stack-vault.sh` and `bash deploy/test/vault-admin.sh` on Linux Docker. Freeze tests first, record the red assertions, and apply the risky mutations against temporary branches. The lead verifies the exact commit and the merged work. No device or Go server is needed in sprint 1.

**Rollout detail.** Steps 3 and 4 use `deploy/up.sh`, a plain Compose shutdown and a second launch. Verify local maintenance in the disposable fixture before handing the contracts and existing launcher to sprint 2.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is D4, D9, D10, R9 and R10, the storage and restricted credential clauses of “Secrets live in OpenBao”, the secret naming rule, the database and vault entries in the compose stack, the C5 fixture connection and C12's database and mutation-hook dependencies, the vault clause of C13, and the four work items in the original brief. [Foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md), [first planning assignment](../../../.foundry/go-rewrite/tasks/plan-go-s1-stack-vault.md).

| Source item | Workstream | Status |
|---|---|---|
| D4, PostgreSQL from the start in Compose | WS-1, WS-2 | Items 1, 2 and 8 prove, build and test database use. |
| D9, server secrets in a vault | WS-3 | Items 4, 5a, 5b and 7 configure and maintain controlled access. |
| D10, OpenBao beside PostgreSQL | WS-1, WS-2, WS-3 | Items 1, 2, 3 and 6 prove and start both services. |
| Vault storage and unseal clauses | WS-1, WS-2 | Items 1 and 3 exercise the restricted storage role and seal file. |
| Vault initial setup and restricted credential | WS-3 | Items 4, 5a and 5b install and publish the role credential. |
| The vault secret name rule | WS-3 | Items 4, 5b and 7 define the initial path and managed policy changes. |
| The compose stack's database and vault entries | WS-2, WS-3 | Items 2, 3 and 6 define local services and launch order. |
| R9, keep secrets out of logs and user surfaces | WS-1, WS-2, WS-3 | Proof redaction and protected file and network boundaries are gated. |
| R10, real PostgreSQL for database tests | WS-1, WS-2, WS-3 | Items 1 and 8 supply real disposable PostgreSQL for the later checks. |
| C5, the fixture database connection | WS-2 | Item 2 supplies the shared authenticated query helper for the later check. |
| C12, the real database and mutation hook | WS-1 | Item 8 supplies the database fixture and Stack's wrapper for the Server mutation script. |
| C13, OpenBao opens on restart | WS-1, WS-2, WS-3 | Items 1, 3 and 6 prove unseal and data retention. Sprint 2 verifies server use. |
| Brief item 1 | WS-1 | The disposable proof gates the other production work. |
| Brief items 2 and 3 | WS-2 | Items 2 and 3 provide PostgreSQL and the sealed OpenBao process. |
| Brief item 4 | WS-2, WS-3 | Items 3, 4, 5a, 5b, 6 and 7 cover seal preparation, bootstrap, startup and credential maintenance. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, “What this project is”, defines the product's parts and contract. | Stack supplies infrastructure and runtime files. The server's vault adapter remains in the Server team's scope. |
| `AGENTS.md`, “Where things are”, keeps new work out of the legacy reference. The epic assigns `deploy/` to Stack. | Production and retained test paths are in `deploy/`. The proof is disposable. The planner or orchestrator updates scouting from the transcript. |
| `AGENTS.md`, habit 1, credits copied code. | The plan uses documented component interfaces and original setup scripts. |
| `AGENTS.md`, habit 2, records problems. | Record the local proof access limitation and the round's verified findings in their assigned artifacts. |
| `AGENTS.md`, habit 3, sends finished work through a pull request. | Exact-commit verification precedes the base-branch handoff. |
| `AGENTS.md`, writing style, keeps private details out of the repository. | Scan the authored files and use redacted proof and test receipts. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on | Can run alongside | Design statements |
|---|---|---|---|---|---|---|---|---|
| 1. Disposable proof | `infra` | `stack/infra` | medium | It covers the two daemons and several restart controls. | Yes | None. | None within this plan. | D4, D10, R9, R10, C13. |
| 8. Fixtures | `infra` | `stack/infra` | medium | It creates isolated test projects and one host-facing test connection. | Yes | Item 1. | None within this plan. | D4, R9, R10, C12. |
| 2. PostgreSQL | `infra` | `stack/infra` | medium | It binds one volume, prepares credentials and reconciles two logins. | Yes | Items 1 and 8. | Item 4. | D4, D10, R9, R10, C5. |
| 3. OpenBao and seal | `infra` | `stack/infra` | medium | It adds one service and a file guard against existing database state. | Yes | Items 1, 2 and 8. | Item 4. | D4, D10, R9, R10, C13. |
| 4. AppRole policies | `infra` | `stack/infra` | medium | A fixture tests provisioning against an initialized vault. | Yes | Items 1 and 8. | Items 2 and 3. | D9, D10, R9, R10. |
| 5a. Initialization material | `infra` | `stack/infra` | medium | It owns the init request and two pre-provisioning checkpoints. | Yes | Item 4. | Item 3 after item 4. | D10, R9, R10, C13. |
| 5b. Credential publication | `infra` | `stack/infra` | medium | It uses the fixed provisioner and owns revocation and publication. | Yes | Items 3, 4 and 5a. | None within this plan. | D9, D10, R9, R10. |
| 6. Launcher | `infra` | `stack/infra` | medium | It reuses the binding API and tests a resumable verified reset for one installation. | Yes | Items 2, 3 and 5b. | None within this plan. | D4, D10, R9, R10, C13. |
| 7. Local maintenance | `infra` | `stack/infra` | medium | It manages one policy and the server role's identifiers through the existing operator identity. | Yes | Items 4, 5b and 6. | None within this plan. | D9, D10, R9, R10. |

Load. Nine test writers, nine builders and nine mutation checks give 27 execution seats. Alternating the two primary vendors gives 14 seats on one and 13 on the other. Each full plan review round uses four seats from the other primary vendor, one from the planner's vendor and one additional logic seat when available. The orchestrator checks the remaining weekly limits at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Round 1 | See the [round 1 register](attack/round-1-findings.md). | The findings were folded or given an explicit outcome. |
| Round 2 | See the [round 2 register](attack/round-2-findings.md). | The findings were folded or given an explicit outcome. The next round reviews the listed patches. |

</details>
