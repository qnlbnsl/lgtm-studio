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

The first check closes the gap between documented features and the exact images, role permissions and process identity used by the stack.

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

Create `deploy/test/vault-testlib.sh`, `deploy/test/with-postgres.sh` and `deploy/test/vault-fixtures.sh`. These belong to Stack. The vault tests read the shared functions `fixture_create`, `fixture_postgres`, `fixture_openbao`, `fixture_exec_as` and `fixture_cleanup`. Tests keep scenario-specific helpers in their own files. The shared library accepts a fixture-owned Compose override and uses unique project and state names with cleanup traps. It does not depend on an item 2 file. (critic-r1 8, logic-r1 9)

`deploy/test/with-postgres.sh -- <command>` starts an independent instance from the pinned PostgreSQL image and uses a temporary filesystem for its data. The host port is assigned by Docker on loopback only. Give the test login `CREATEDB` and a disposable password. Run the command with `LGTM_TEST_POSTGRES` in its environment, then remove the container. The script writes the variable for the Server team's store tests and `deploy/test/checks.sh --only C12` to read. The consumer needs no changes to its connection-string format. The production `postgres` service remains unpublished. **UNREVIEWED MECHANISM** (architect-r1 2). A command wrapper gives host tests the disposable database without adding a database service or printing the password in a connection string.

Freeze the following assertions in `deploy/test/vault-fixtures.sh`. A stub that reuses a project name or omits the database-creation privilege supplies a red test.

- `two_fixture_projects_clean_up` starts two projects as an ordinary Docker operator. It asserts distinct names and that teardown leaves neither project's resources or state.
- `test_database_is_disposable` invokes a host PostgreSQL client through the wrapper. It asserts authenticated database creation, child-only connection settings, loopback port publication, and cleanup when the child fails. It inspects Docker for the fixture port while the child runs.

The item is risky because test setup handles credentials and creates isolated resources. Publish its port on every interface as the mutation. The network assertion must fail. Acceptance is `bash deploy/test/vault-fixtures.sh` reporting `ok` for both assertions.

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

Create `deploy/compose.yaml`, `deploy/.gitignore`, `deploy/postgres-init.sql`, `deploy/stack-prepare.sh` and `deploy/test/postgres.sh`. Stack writes these files. Item 3 extends the Compose and prepare files after item 2 merges. Item 6 runs the two preparer commands below. (logic-r1 9)

- Pin PostgreSQL to `postgres:16.16-alpine@sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d` for the daemon and the preparer. Item 1 confirms the digest on the build platform before merging. Use the named volume `pgdata` with `PGDATA=/var/lib/postgresql/data/pgdata`. Keep the volume's installation binding at `/var/lib/postgresql/data/lgtm-installation`, outside the PostgreSQL data directory. The preparer writes it and the launcher checks it before starting services. (logic2-r1 6, logic2-r1 8, threat-r1 6)
- The Compose project defaults to `lgtm`. The default state directory is `./.state` relative to the Compose file and is ignored by Git. `LGTM_STACK_STATE_DIR` overrides it with an absolute path. Production file binds use long syntax and `create_host_path: false`. The preparer refuses a directory at a file path and names the path and matching-state restore step. (architect-r1 10, critic-r1 10)
- `stack-prepare prepare-db` claims the installation using item 6's binding protocol. It makes one password at a time. It retains each valid existing password and fills in missing files only before `PG_VERSION` exists. A live cluster with absent or invalid credentials returns `restore the matching PostgreSQL password files`. File writes use a private temporary file and an atomic rename. Each password is an independent random 32-byte value encoded as 64 lowercase hexadecimal characters and one newline.
- Start `postgres` with `restart: unless-stopped` and `POSTGRES_PASSWORD_FILE`. Set `POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 --auth-local=scram-sha-256` and `POSTGRES_HOST_AUTH_METHOD=scram-sha-256`. Salted Challenge Response Authentication Mechanism (SCRAM) protects loopback and socket connections as well as traffic from another container. Pass mounted password values to `psql` without putting them in arguments or output. (logic-r1 4, logic2-r1 3, threat-r1 4). **UNREVIEWED MECHANISM** (critic-r1 5). Enforcing passwords at initialization preserves the authenticated readiness requirement.
- `stack-prepare reconcile-db` waits up to 60 seconds for an authenticated administrator connection to the final network listener. The launcher runs this command only after a volume claim is accepted. It runs `postgres-init.sql` as a retryable setup script. The image initializes the cluster. The reconciler owns application role and database creation, so retries do not depend on one-time image hook execution. Mark completed reconciliation as `pg.reconciled` in the state root after both network queries pass. The preparer writes it and the launcher reads it for diagnostics. Restore a corrupt cluster through the documented local restore path. (architect-r1 5, logic2-r1 4, critic-r1 3)
- Create missing logins with passwords from the retained files. A mismatched password on an existing login is a repair diagnostic rather than a silent rotation. Create `lgtm_app` with `OWNER lgtm_app` and `lgtm_bao` with `OWNER lgtm_bao`. These logins have neither `CREATEDB` nor `CREATEROLE`. The reconciler verifies the owners on existing databases. Revoke public connect on those databases, `postgres` and `template1`. Revoke public schema creation and leave each database owner able to create its own tables. OpenBao creates its storage table without an extended grant or a precreated-table setting. (architect-r1 8, logic-r1 1, critic-r1 1, threat-r1 5)
- Health executes authenticated `SELECT 1` over loopback for both role files. Use a five-second interval, a three-second timeout, 10 retries and a five-second start period for both services. A role setup wait or readiness failure exits with a phase-specific message.
- Create `data` and `vault` with `internal: true`. The database joins `data`. It publishes no port. Set `cap_drop: [ALL]` and `no-new-privileges:true` for each sprint 1 service. Add back only file setup and user-switch capabilities needed by the pinned entries. Verify those capability lists during the first clean run. The later server image must omit raw-network capability. (threat-r1 7)

| State file | Numeric owner and group | Mode | Writer and reader |
|---|---|---|---|
| `postgres-admin.password` | 70 and 70 | `0400` | `prepare-db` writes. PostgreSQL and the reconciler read. |
| `postgres-app.password` | 70 and 10001 | `0440` | `prepare-db` writes. The reconciler, readiness probe and later server read. |
| `postgres-bao.password` | 70 and 10002 | `0440` | `prepare-db` writes. The reconciler, readiness probe and OpenBao read. |

Mount PostgreSQL's three files by basename at `/run/lgtm-db/`. Use the same files for authenticated reconciliation and readiness. Keep the health probe's passwords in the subprocess rather than container settings.

Freeze these tests in `deploy/test/postgres.sh`. Use a fixture Compose override with missing grants, a trusted loopback rule or a disposable volume that is not retained to reach the red assertions.

- `postgres_roles_are_isolated` connects from another test container. It checks database ownership, scratch writes, denied cross-database reads including the vault's storage table, and denied role or database creation. It runs OpenBao using the disposable proof setup and requires the storage table to appear under the vault login.
- `postgres_waits_for_both_databases` starts the database daemon before invoking reconciliation and checks the unhealthy state. After reconciliation it changes one mounted fixture password while leaving the login enabled. Both the health command and the affected network login must fail. Restoring the password returns health. No sleep or hook inside the production initialization script is necessary. (critic-r1 2)
- `postgres_has_no_trust_rules` reads the effective host authentication rules and verifies the two wrong-password controls.
- `postgres_repairs_half_initialized_volume` starts a fixture with the original password files and only `lgtm_app` created. It writes a sample row, replaces the container with the same volume, and runs reconciliation. Assert creation of `lgtm_bao`, retention of the sample and password files, and a no-op second reconciliation.

The item is risky. Removing the initialization authentication flags must fail the wrong-password or rule assertion. Granting public connection to the vault database must fail the access assertion. Skipping reconciliation must fail the partial-volume test. Acceptance is `bash deploy/test/postgres.sh` with four passing assertions.

**Work item 3. Prepare the seal key and start OpenBao**

Create `deploy/openbao.hcl`, `deploy/openbao-start.sh` and `deploy/test/seal.sh`. Extend `deploy/compose.yaml` and `deploy/stack-prepare.sh`. Mount the configuration read-only at `/openbao/config/lgtm.hcl`. Use `openbao/openbao:2.7.3@sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9`, subject to item 1's confirmation. (architect-r1 7, logic-r1 7, logic-r1 8)

- `stack-prepare seal` authenticates to `lgtm_bao` as its owner. It checks `to_regclass('public.openbao_kv_store')` and, if present, checks for rows. On any query error it stops before writing a key. A row, bootstrap journal, or published credential is sufficient to refuse creation. The missing-key diagnostic is `seal key missing for existing vault`. The preparer also creates the empty private mount directories described in item 6. (logic-r1 6)
- `bao/unseal.key` is a single regular 32-byte file with owner and group 10002 and mode `0400`. Mount the file read-only at `/run/lgtm-bao/unseal.key`. OpenBao reads it as user and group 10002 under a `seal "static"` block with key identifier `lgtm-1`. The key preparer preserves an existing valid file byte-for-byte.
- Set Compose `entrypoint` to `/bin/sh /openbao-start.sh` and `command` to `server`. Stack writes the wrapper and OpenBao consumes its arguments. The wrapper reads only the mounted vault database password, builds `BAO_PG_CONNECTION_URL` with `sslmode=disable`, and executes `bao server -config=/openbao/config/lgtm.hcl`. The explicit wrapper bypasses the image's development default and does not need the memory-lock capability or the unsupported `disable_mlock` setting. Use `storage "postgresql"` with `max_connect_retries=0`. (logic-r1 7, critic-r1 11)
- Use a Transmission Control Protocol (TCP) listener on the container interfaces at port 8200. Transport Layer Security (TLS) is disabled on these internal networks. OpenBao joins `data` and `vault`, publishes no port, and declares a dependency on healthy `postgres`. Its read-only credential mounts contain only the seal key and its database password. Set `BAO_ADDR` to loopback inside the service so `bao status -format=json` works for `vault_ready` in `deploy/test/checks.sh`.
- Report Compose health using initialized, unsealed and PostgreSQL storage status. The launcher distinguishes observable decryption failure and authentication refusal using fixed classifications. It never streams raw logs containing a connection string. If the cause is unavailable, print `OpenBao did not become active` with the failed phase. (logic-r1 6)

Freeze these tests in `deploy/test/seal.sh`. A preparer stub that leaves the key absent or accepts a published host port has a failing assertion.

- `seal_key_is_created_once` compares the original file's type, length, mode, ownership and fingerprint across preparation and restart. The privileged fixture helper performs the file inspection.
- `missing_key_preserves_existing_vault` seeds a fixture record, stops the vault, and stashes the key through the fixture helper. Assert the named refusal, no replacement key and preserved rows. Restore the key and read the original value.
- `wrong_key_preserves_existing_vault` installs a different 32-byte file with the same mode. Assert no healthy or unsealed state and an unchanged database. Reinstall the original file and read the record.
- `seal_refuses_with_journal_and_empty_database` and `seal_refuses_with_credential_and_empty_database` seed only the named local evidence. Each asserts that no key is generated and setup returns the missing-key diagnostic. (logic-r1 6)
- `openbao_runs_with_postgres_storage` requires `storage_type=postgresql` and checks the owner-created table.
- `vault_has_no_host_port` and `stack_services_have_no_egress_and_no_net_raw` inspect runtime port bindings, the declared internal networks and capability sets. Use an isolated control endpoint and test-owned probe on each network. The expected result is no external route and no raw-network capability in the services.

The item is risky. Mutate the guard to check database rows alone. The journal and credential controls must fail. Use the default development command as a second mutation. The storage-type test must fail. Acceptance is `bash deploy/test/seal.sh` with the eight named results passing.

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

Create `deploy/openbao-tools/Dockerfile`, `deploy/openbao-tools/.dockerignore`, `deploy/.dockerignore`, `deploy/openbao-server-policy.hcl`, `deploy/openbao-admin-policy.hcl`, `deploy/openbao-provision.sh` and `deploy/test/vault-policy.sh`. Build only from the isolated `./openbao-tools` context. The Dockerfile has no project file copy. The context allows only the Dockerfile and ignore file. The outer ignore also excludes `.state`. Pin the helper's base to item 3's digest and pin the `curl` and `jq` package versions at the exact-commit build. Build it only when the expected helper image is missing. (architect-r1 7, critic-r1 10, logic-r1 8, threat-r1 6, threat-r1 8, logic2-r1 8)

- Enable the version 2 key-value engine at mount `lgtm` and enable AppRole at `auth/approle`. Create role and policy `lgtm-server`. Bind a reusable secret identifier using `secret_id_ttl=0` and `secret_id_num_uses=0`. Set `token_type=batch`, `token_ttl=15m`, `token_max_ttl=15m` and `token_no_default_policy=true`. Grant read, create and update at `lgtm/data/server/link/signing-key`. Another server path needs a policy change through item 7. (architect-r1 4)
- Create role and policy `lgtm-admin` and a separate protected credential. It issues nonrenewable 15-minute batch tokens with no default policy. Its direct access is limited to reading and updating `sys/policies/acl/lgtm-server`. It has `read` on `auth/approle/role/lgtm-server/role-id`, `list` on the role's `secret-id`, and `update` on its `secret-id/lookup`, `custom-secret-id` and `secret-id-accessor/destroy`. The bootstrap administrator creates the roles. The operator uses these exact policy paths. Keep the local operator credential under `admin/operator.json` with owner and group 0 and mode `0600`. Stack provisioning writes it. The local maintenance command alone uses it after bootstrap. It is privileged because editing the server's policy can widen its access. **UNREVIEWED MECHANISM** (threat-r1 1, logic2-r1 2). A narrowly scoped management role provides policy and credential maintenance without granting root-generation access or access to the stored data.
- The provisioner reads the temporary root token from `/state/admin/root.token` and reads its staged IDs from the protected journal directory. For retry of an individual role, keep the candidate identifier durable before registration. A successful lookup with a matching accessor means present. The frozen test measures the absent-identifier response on the pinned version and fixes its exact status and safe error shape as the only secondary condition that permits creating it. **UNREVIEWED MECHANISM** (threat-r1 9). Measure the response before dispatch instead of assuming its shape from an unmatched probe. Storage, authorization and transport failures stop the run. Validate both credentials before marking provisioning complete. Stack writes and the next work item consumes the protected pending JSON files. The HTTP requests take values through files or input streams. (threat-r1 9)
- Completed bootstrap preserves its installed policy. Item 7 applies explicit policy updates. The initial exact-path policy does not authorize the server to discover other vault paths.

Freeze tests in `deploy/test/vault-policy.sh`. Use the shared proof fixture without waiting for work item 2. An empty fixture policy makes the positive access assertion fail.

- `app_role_access_is_scoped` verifies a nonrenewable 900-second lease, create/read/update on the signing-key path, and denial of sibling reads and writes, tool reads and writes, listing metadata, policy changes and token creation. An attempted second compare-and-set create must conflict.
- `app_role_can_log_in_again` verifies two distinct logins from the same file, denial for a wrong identifier, and reuse of the same accessor after provisioner replay.
- `operator_role_is_separate` verifies only the operator can manage server identifiers and its policy. The operator token cannot read a stored secret or generate a root token. Validate denial at the API and verify the mount inventory.
- `secret_id_lookup_classifies_absence` records the pinned response for an unregistered candidate. Verify a valid lookup reuses the accessor. Inject a revoked-token response and a transport failure and verify neither triggers registration.
- `images_are_pinned_by_digest` checks the image references and the helper base against the proof digests once Compose is merged. The item-4 check covers its Dockerfile. The item-6 merge check covers the rendered stack.
- `helper_context_excludes_state` builds a disposable helper with sentinel fixture material under `.state`. The build input and layers must exclude that material.

The item is risky. Broaden the server policy and expect the sibling test to fail. Set the runtime identifier to one use and expect the second-login test to fail. Acceptance is `bash deploy/test/vault-policy.sh` with six passing assertions.

**Work item 5a. Initialize OpenBao and save bootstrap material**

Create `deploy/openbao-bootstrap.sh` and `deploy/test/vault-bootstrap.sh`. This script owns the initialization checkpoints. Item 5b invokes it in the setup container and consumes the material it produces. Split the two scripts so each builder has its own frozen assertions. (critic-r1 7)

Use one versioned local checkpoint file `admin/bootstrap.state`, mode `0600`, owned by root. Stack's bootstrap and publication scripts write it by atomic replacement. The prepare helper and launcher read it. Its step is one of `intent`, `initialized`, `provisioned`, `revoked` or `published`. A stage advances only after its inputs and outputs are durable. Each private file and its containing directory are flushed before advancing. (logic-r1 2, threat-r1 9)

Query initialization with `GET /v1/sys/init`. Before the initial request record the `intent` checkpoint. Send `PUT /v1/sys/init` with `recovery_shares=1` and `recovery_threshold=1`. Save the response directly to a private response file and flush before parsing. Save `recovery_keys_base64[0]` to `admin/recovery.key` and `root_token` to `admin/root.token`. Both have mode `0600`. Advance to `initialized` and then remove the response JSON. If the response has been lost after initialization, setup follows the state table in item 5b. (threat-r1 2)

Treat initialization and readiness separately. Poll `GET /v1/sys/health` for status 200 before provisioning or validation, including after a successful init. Status 503 means wait within the same 60-second elapsed-time budget. Set individual request timeouts to the time remaining. (logic2-r1 7)

Freeze assertions in `deploy/test/vault-bootstrap.sh`. A stub that advances without writing durable material makes the file and resume assertions fail. The test runs a test-owned recording proxy on `vault` and points only the fixture's `BAO_ADDR` at it. The proxy forwards to real OpenBao. It can hold a response before delivery and report a boundary signal using an isolated test file. The test kills the fixture job through Docker when the proxy holds the selected response. For local write boundaries, a fixture-only command shim reports the response-file and checkpoint renames. The test kills the fixture job after the named rename. (critic-r1 2)

- `bootstrap_saves_material_before_continuing` interrupts setup with a saved response and then at the initialized checkpoint. It verifies root and recovery material survive before the next request and that the second run performs no extra init.
- `bootstrap_refuses_incomplete_state` lets the server commit initialization, drops the response, and kills the job. The second run must emit `vault is initialized but bootstrap state is incomplete` and keep the records. The launcher tests exercise the guarded operator reset.
- `bootstrap_waits_for_active_vault` holds health at 503 before allowing the 200 response. Assert that no provisioning request goes through before 200.

The item is risky. Move the checkpoint update ahead of the material write and expect the first assertion to fail. Treat initialization as readiness and expect the third assertion to fail. Acceptance is `bash deploy/test/vault-bootstrap.sh` with three passing assertions.

**Work item 5b. Publish the credential and verify completed startup**

Create `deploy/openbao-init.sh` and `deploy/test/vault-init.sh`. Add the `openbao-init` one-shot service under Compose profile `setup`. It uses the helper image and joins `vault`. Set its `BAO_ADDR` from service `openbao` and port 8200. Mount `admin/` and `server/` separately at `/state/admin` and `/state/server`. Mount both initialization scripts, the provisioner and the policies read-only. Normal setup has no seal key or database password mount.

The following table is evaluated in order. An unreadable probe stops startup before changes. A confirmed storage-binding mismatch in item 6 stops before this job. (logic-r1 2, architect-r1 6, threat-r1 3, threat-r1 9)

| OpenBao and local state | Outcome and observation |
|---|---|
| Uninitialized, no checkpoint or an `intent` checkpoint with neither recovered material nor credential | Initialize once. Resume from the successful response. `bootstrap_saves_material_before_continuing`. |
| Uninitialized and a later checkpoint, administrative material or a published file exists | Print `state directory is ahead of the database`. Keep the files and volume. `stack_refuses_state_ahead_of_database`. |
| Initialized, absent or `intent` checkpoint, a complete private response exists, and published file is absent | Finish saving and checking the response. Advance to `initialized`. `bootstrap_saves_material_before_continuing`. |
| Initialized, absent or `intent` checkpoint with no complete response, or a checkpoint whose required material is missing | Print `vault is initialized but bootstrap state is incomplete`. Keep the database. The guide gives the restore or explicit reset path. `bootstrap_refuses_incomplete_state`. |
| Active, `initialized`, trusted root material and no published file | Provision or resume the two staged identities. Verify both logins. Save `provisioned`. `bootstrap_resumes_staged_registration`. |
| Active, `provisioned`, pending identities verified | Revoke the initial token using `POST /v1/auth/token/revoke-self`. Confirm 403 on its lookup. Persist the `revoked` checkpoint before deleting the token file. On retry, an already-revoked token can complete this transition when the two identities verify. `bootstrap_revokes_root_before_publish`. |
| Active, `revoked`, valid pending objects and no published file | Validate the saved objects and publish within the destination mount. Advance to `published`. `bootstrap_resumes_at_publication_boundary`. |
| Active, `revoked` with a valid final file, or `published` with a valid final file | Verify the exact file shape and ownership. Authenticate once with the server role. Advance the checkpoint if needed and remove leftover staging. Print `vault already configured` on a completed rerun. `bootstrap_is_idempotent`. |
| Active, `published` with missing, malformed or rejected server credentials | Print `server vault credential needs repair`. The operator can run the rotation command in item 7. Preserve the vault and local administrator credential. `bootstrap_refuses_invalid_published_credential`. |
| Any other conflicting state | Print `vault is initialized but bootstrap state is incomplete` and preserve the evidence for local recovery. |

Copy the validated server JSON to a private hidden file inside `/state/server`. Apply final ownership and mode. Flush the hidden file, rename it to `/state/server/openbao.json` within the same mount and flush that directory. Keep the pending credential, recovery material and revocation checkpoint until publication and the `published` checkpoint are durable. Remove transient material afterwards. A crash after the rename must leave either a resumable `revoked` state or a complete published file. (logic2-r1 1, architect-r1 3). **UNREVIEWED MECHANISM** (logic2-r1 5). Retain pending state until the final rename is durable, then use the same single-login validation on recovery and on an ordinary rerun. This rejects a published file that no longer matches restored storage.

**Contract for sprint 2** (architect-r1 9)

| Surface | Stack writes or supplies | Sprint 2 reads or extends |
|---|---|---|
| Container user | The Stack team sets numeric user and group 10001 in the server image. | The server process uses the mounted credential files as that user. |
| Compose names | Stack publishes no host port for `postgres` or `openbao`. PostgreSQL uses `data`, and OpenBao uses `data` and `vault`. | Stack joins the server to both internal networks. The web proxy stays on its own server-facing network. |
| Database | Stack mounts `postgres-app.password` read-only at `/run/lgtm/postgres.password`, retaining owner 70, group 10001 and mode `0440`. Stack sets `LGTM_POSTGRES_PASSWORD_FILE` to that path. It provides the host `postgres`, port 5432, name and user `lgtm_app`. | The server settings consume the file as one hexadecimal password and trailing newline. The teams agree on the remaining setting names for the fixed connection values in sprint 2. |
| Vault credential | The initializer writes default `deploy/.state/server/openbao.json`, or `server/openbao.json` under `LGTM_STACK_STATE_DIR`. It is 8-bit Unicode Transformation Format (UTF-8) JavaScript Object Notation (JSON) with exactly the two nonempty string fields `role_id` and `secret_id` and one trailing newline. Owner and group are both 10001, mode `0400`. Stack mounts it read-only at `/run/lgtm/openbao.json`. | The Server team reads `LGTM_OPENBAO_CREDENTIAL_FILE` and `LGTM_OPENBAO_ADDR`. Stack sets them to the mount path and the HTTP service `openbao` on port 8200. The server signs in at `POST /v1/auth/approle/login`, uses `auth.client_token` in `X-Vault-Token`, and reauthenticates before its lease expires. |
| Signing key | Stack grants access to `lgtm/data/server/link/signing-key` in OpenBao. | The Server team writes and reads `private_seed_b64`, a standard base64-encoded 32-byte Ed25519 seed. The first write uses `options.cas=0`. A creation conflict leads to a read of the stored key. |
| Launcher | Stack writes executable `deploy/up.sh` in sprint 1. | The sprint 2 Stack plan extends that file. It starts the server only after OpenBao bootstrap succeeds. An atomic credential replacement needs a new server container to bind the new inode. |

Freeze these tests in `deploy/test/vault-init.sh`. A stub that claims success without publishing a file reaches red assertions. In the disposable fixture, the proxy holds and exposes status at the root revocation boundary. To test local crash points deterministically, the test overrides the job's command lookup with a fixture-only `mv` executable. It delegates normal renames and sends a fixture-ready signal before or after the rename to `/state/server/openbao.json`. The test kills the container at that signal. Production calls the ordinary system command and contains no test flag. The tests use the real separate mount layout. (critic-r1 2, architect-r1 3)

- `bootstrap_revokes_root_before_publish` asserts the captured initial token gets 403 immediately before the file appears. The positive file can log in and has the specified owner, group, mode and shape.
- `bootstrap_resumes_staged_registration` interrupts just before and just after identifier registration using the fixture proxy. A rerun registers one identifier per role and publishes once.
- `bootstrap_resumes_at_publication_boundary` runs the revoke and the before/after-rename interruptions in distinct fixtures. Each retry leaves one valid credential. Test observations include no captured credential in logs or container configuration.
- `bootstrap_is_idempotent` compares key and credential fingerprints and modification times. The proxy observes one server login, no provisioning requests, and no change to state on a clean second run. After an interrupted finalization it observes cleanup without replaying provisioning.
- `bootstrap_refuses_invalid_published_credential` supplies missing, malformed and revoked server files. It verifies the repair diagnostic and unchanged vault contents.

The item is risky. Remove root revocation, move the pending-file cleanup before publication, or use a cross-mount rename, one per mutation. Each must fail the corresponding assertion. Acceptance is `bash deploy/test/vault-init.sh` with five passing assertions.

**Work item 6. Start and verify the sprint 1 stack with one command**

Create executable `deploy/up.sh`, `deploy/README.md` and `deploy/test/stack-vault.sh`. Extend `deploy/stack-prepare.sh` for binding, privileged inspection and reset. The default invocation from the repository root is `deploy/up.sh`. The existing check script calls it directly. Use one Test Anything Protocol (TAP) line per named assertion in the retained suites, with `ok` or `not ok`, an ordinal and the test identifier. (critic-r1 11)

The state root belongs to the invoking operator with mode `0700`. The `bao/`, `admin/` and `server/` directories belong to root with mode `0700`. The same privileged helper that created those directories checks or cleans them. Provide a fixture-only Compose override for stashing and restoring the seal file and reading fingerprints. The production maintenance helper handles permitted inspection and reset without printing private file contents. A Docker-capable operator can run it without a host `sudo` step. Any manual restoration of root-owned files needs host administrator privileges or the documented helper. (critic-r1 4)

The launcher locks the state root with `flock`. Use a local Docker daemon and a local filesystem. Under that lock it claims the named project's volume before changing service mounts. Write `installation.id` in the state root and atomically link a candidate binding into the volume's `lgtm-installation`. The value binds the project, a random installation identifier and a fingerprint of the canonical state path. Serialize the claim with an atomic link on the single project volume. Reject unclaimed volumes containing PostgreSQL data. A failed claimant can neither start nor recreate services. A competing root for one Docker Compose project can claim the volume at most once. Losing claimants return `Compose project belongs to another state directory` before updating live services. A missing volume and an existing claimed state return `state directory is ahead of the database`. A retained volume and missing state return `restore the matching installation state`. The launcher writes the local binding and the prepare helper writes its paired volume binding. **UNREVIEWED MECHANISM** (logic2-r1 6). Atomic claiming on the selected Docker daemon plus the local lock enforces the same serialization without a separate host-global lock. Write a provisional local identity before attempting the volume link. Commit it locally after the matching volume link exists. A retry with a matching provisional identity commits it. A missing marker after a committed binding fails closed.

Run these phases under the lock. Emit `phase <name> start` and `phase <name> done` at each phase boundary. The launcher writes these fixed names and the frozen fixture reads them. (critic-r1 2)

| Phase | Operation |
|---|---|
| `database` | Run `stack-prepare prepare-db`. Start `postgres` without waiting for health. Run `stack-prepare reconcile-db`, then wait for both role queries. |
| `seal` | Run `stack-prepare seal`. It refuses inconsistent storage or files before OpenBao starts. |
| `vault` | Start `openbao`. Build the pinned tools image only if absent. Run `openbao-init` and wait for both daemons with `docker compose -f deploy/compose.yaml up -d --wait --wait-timeout 60 postgres openbao`. |
| `ready` | After success, print `postgres and openbao are healthy`. A failed earlier phase exits with its specific diagnostic. |

Use `docker compose -f deploy/compose.yaml run --rm --no-deps <job>` for one-shot jobs. `deploy/up.sh` is the supported first-start command. Compose can recreate already initialized daemons without running setup. A plain Compose `down` preserves the named volume and state. For configured installations supply the same `COMPOSE_PROJECT_NAME` and `LGTM_STACK_STATE_DIR` for maintenance. (architect-r1 10)

**Operator recovery** (logic-r1 3, threat-r1 2, threat-r1 3, critic-r1 3, architect-r1 6)

| Diagnosis | Operator action in `deploy/README.md` |
|---|---|
| A partial PostgreSQL setup still has matching files and a valid cluster. | Rerun `deploy/up.sh`. Reconciliation fills in missing roles and databases and retains the existing data. |
| The volume survives but local credentials or the seal key are lost. | Stop the stack with `docker compose -f deploy/compose.yaml down`. Restore the matching local file at its documented path, owner and mode. Rerun `deploy/up.sh`. |
| The state is ahead of the database, or the database is an older copy that refuses the published credential. | Restore the matching local database and state pair if available. For a valid installation with just a lost or revoked server file, use `deploy/vault-admin.sh rotate-server-credential`. |
| Initialization committed but its local response is unusable, or matching material is otherwise unavailable. | Preserve the existing installation while diagnosing. To discard it explicitly, use `deploy/up.sh --reset --yes` with its original project and state settings. This removes the selected PostgreSQL volume, including application and vault data, and deletes local credentials and key files. Run `deploy/up.sh` afterwards for a new installation. |

**UNREVIEWED MECHANISM** (logic-r1 3, threat-r1 2, critic-r1 3). The reset is an explicit installation reset. An unauthenticated state or row count cannot prove an old database contains only setup rows. Ordinary startup never deletes data. The reset verifies the binding and the selected project, requires `--yes`, stops the two daemons, removes only that project's verified PostgreSQL volume, then removes its local state contents through `stack-prepare reset-state`, which mounts local state without referencing the deleted volume. Preserve the lock until reset exits. Stack writes and the operator reads the two reset arguments and the listed diagnostics. A mismatch refuses reset against the foreign installation. The guide warns that a checkout cleanup may remove ignored state and documents a separate local state directory through the existing override.

Freeze tests in `deploy/test/stack-vault.sh`. A launcher stub that succeeds without running setup fails on readiness. A test-only override holds the first launcher inside the `seal` phase using an isolated file signal before the second attempts entry. Use the phase lines to record ordering.

- `stack_reopens_existing_vault` checks that the launcher is executable, starts both healthy services, persists a fixture signing key, runs ordinary Compose `down`, and verifies the same key after restart through the mounted credential as user 10001.
- `stack_refuses_missing_key` stashes the key through the fixture helper after persisting data. Assert the missing-key diagnostic, no replacement key and no healthy vault. Restore the original and read the record.
- `stack_bootstrap_is_serialized` overlaps two invocations using the same state root. It checks the phase boundaries and one retained identity.
- `stack_refuses_foreign_state_root` overlaps two invocations with distinct roots for one Compose project and also reruns the loser sequentially. Require the binding diagnostic and prove that the winner's existing record and published credential remain usable.
- `stack_refuses_state_ahead_of_database` removes the fixture volume while preserving local state. Startup must report the mismatch and preserve local files. A separate fixture keeps the volume while removing local state and must report the restore diagnostic.
- `stack_reset_requires_confirmation` uses a disposable incomplete bootstrap fixture. Assert the command without `--yes` leaves both stores intact. Assert the confirmed command removes that installation and a fresh startup passes. Keep a second fixture running and verify it survives the reset.

The item is risky. Remove the state lock, bypass the volume binding, or ignore a failed seal step. Expect the respective serialization, foreign-root, or missing-key test to fail. Acceptance is `bash deploy/test/stack-vault.sh` with six passing assertions.

**Work item 7. Maintain the server credential from the local stack**

Create `deploy/vault-admin.sh`, `deploy/openbao-admin.sh`, and `deploy/test/vault-admin.sh`. Extend `deploy/README.md` after item 6 merges. The host launcher uses the same state lock and project validation as startup. A one-shot tools container on `vault` reads the operator credential and writes the server file. Only the setup and maintenance jobs mount `admin/`. (threat-r1 1, logic2-r1 2, critic-r1 9)

- `deploy/vault-admin.sh apply-server-policy` reads the reviewed `deploy/openbao-server-policy.hcl` and applies an intentional change to that named policy. The operator reviews the file before running the command. This is the wave 2 handoff for adding approved server paths. The command does not change the role or other policies. The initial narrow policy still applies at the end of sprint 1. **UNREVIEWED MECHANISM** (architect-r1 4). An explicit local policy update avoids widening runtime access by default or doing a privileged change during ordinary startup.
- `deploy/vault-admin.sh rotate-server-credential` logs in with the local operator AppRole. Journal the candidate and superseded accessor in a root-only file before publication. Register and verify the new login, then publish with the same within-mount rename as item 5b. Recreate the server container if it exists. If recreation fails, leave the prior identifier active and resume with the durable new file and journal. Once the recreated container has the new mount, destroy the previous identifier by accessor. When the server file has been lost, list the server role's accessors and record those to revoke before registering a replacement. Retrying uses the same candidate and resumes pending revocation. Existing batch tokens can remain valid for their remaining 15-minute lease.
- The authenticated operator login is what provides maintenance access. The recovery share by itself cannot mint a root token on this image with the chosen listener settings. Keep its file protected. A lost operator identity and unusable matching state require the restore or explicit reset procedure. Sprint 1 tests use a disposable reader container at user 10001 in place of the future server binary.

Freeze tests in `deploy/test/vault-admin.sh`. A command stub that returns success without registering or publishing a new credential makes these assertions fail.

- `admin_can_rotate_and_widen_policy_after_root_revocation` bootstraps once, verifies the original root token is denied, and applies a fixture policy that adds one server-owned path. The new file logs in, reads the original signing key and reaches the added path. Tool and administrative paths remain unavailable. The superseded identifier cannot log in. A test reader with a fresh bind mount reads the new inode.
- `admin_resumes_credential_rotation` interrupts after publication and before destroying the old identifier by making the test Docker wrapper refuse reader recreation. A second run reuses the staged identity, recreates the reader, and retires the old accessor. It verifies that the vault record is unchanged.

The item is risky. Skip old-accessor revocation or skip the reader recreation and expect the relevant assertion to fail. Acceptance is `bash deploy/test/vault-admin.sh` with both assertions passing.

**Post-build checks**

These checks cover behavior that needs the production files or the sprint 2 stack. The round 1 register maps them to the reports.

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

The source universe is D4, D9, D10, R9 and R10, the storage and restricted credential clauses of “Secrets live in OpenBao”, the secret naming rule, the database and vault entries in the compose stack, C12's database fixture dependency, the vault clause of C13, and the four work items in the original brief. [Foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md), [first planning assignment](../../../.foundry/go-rewrite/tasks/plan-go-s1-stack-vault.md).

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
| C12, the real database dependency | WS-1 | Item 8 gives the Server team's tests a disposable host database. |
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
| 2. PostgreSQL | `infra` | `stack/infra` | medium | It separates setup credentials, role grants and authenticated readiness. | Yes | Items 1 and 8. | Item 4. | D4, D10, R9, R10. |
| 3. OpenBao and seal | `infra` | `stack/infra` | medium | It adds one service and a file guard against existing database state. | Yes | Items 1, 2 and 8. | Item 4. | D4, D10, R9, R10, C13. |
| 4. AppRole policies | `infra` | `stack/infra` | medium | A fixture tests provisioning against an initialized vault. | Yes | Items 1 and 8. | Items 2 and 3. | D9, D10, R9, R10. |
| 5a. Initialization material | `infra` | `stack/infra` | medium | It owns the init request and two pre-provisioning checkpoints. | Yes | Item 4. | Item 3 after item 4. | D10, R9, R10, C13. |
| 5b. Credential publication | `infra` | `stack/infra` | medium | It uses the fixed provisioner and owns revocation and publication. | Yes | Items 3, 4 and 5a. | None within this plan. | D9, D10, R9, R10. |
| 6. Launcher | `infra` | `stack/infra` | medium | It sequences fixed phases and guards start and reset for one installation. | Yes | Items 2, 3 and 5b. | None within this plan. | D4, D10, R9, R10, C13. |
| 7. Local maintenance | `infra` | `stack/infra` | medium | It manages one policy and the server role's identifiers through the existing operator identity. | Yes | Items 4, 5b and 6. | None within this plan. | D9, D10, R9, R10. |

Load. Nine test writers, nine builders and nine mutation checks give 27 execution seats. Alternating the two primary vendors gives 14 seats on one and 13 on the other. Each full plan review round uses four seats from the other primary vendor, one from the planner's vendor and one additional logic seat when available. The orchestrator checks the remaining weekly limits at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings | Outcome |
|---|---|---|
| Round 1 | See the [round 1 register](attack/round-1-findings.md). | The findings were folded or given an explicit outcome. The next round reviews the listed patches. |

</details>
