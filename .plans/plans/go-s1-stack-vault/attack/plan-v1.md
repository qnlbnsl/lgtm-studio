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

Sprint 1 gives LGTM Studio a database and a vault that can run on their own. One command starts both services. PostgreSQL keeps the application database and a separate database for the vault. OpenBao uses a key file to open the vault after a restart, so routine startup needs no manual unseal.

The first build item uses disposable containers to prove that OpenBao puts its data in PostgreSQL and can reopen that data from the same key file. Access to Docker was denied during planning. The result of that first build item determines whether the rest of this plan can proceed.

The server in sprint 2 will read a credential from a protected file. That credential gives it access to its own signing key in the vault. Sprint 1 checks the credential with OpenBao directly, so its checks can run before the server exists.

## Key decisions

The installer makes the vault's unseal key once. If the file is missing and earlier vault data exists, startup stops and reports the missing file. Restoring the original key lets OpenBao reopen the data.

The server credential is reusable until revoked. OpenBao exchanges it for a session token that lasts 15 minutes. The initial policy grants access to one signing-key path. The bootstrap administrator's token is revoked before the credential is handed to the server.

[Full decision log](decisions.md)

## Workstreams

### Prove storage and restart behavior

The disposable proof writes a sample secret to OpenBao. It checks that PostgreSQL gains records, replaces the OpenBao container, and reads the same secret after restarting PostgreSQL. It also checks that taking away the key prevents the vault from opening.

### Start PostgreSQL and open the vault inside the stack

The stack gives PostgreSQL two databases and restricts each application login to its own database. It checks that PostgreSQL accepts real connections before starting OpenBao. OpenBao reports healthy once an initialized vault is open. If an installation's key is missing, startup stops before opening the vault and reports what needs to be restored.

### Hand off a restricted server credential

The first-start script configures the server's access and writes its credential file. Setup preserves enough private information to resume a partially completed run when the necessary data was saved. A completed run keeps the existing credential file and vault configuration. The tests verify that the credential can write and read the signing-key path and is denied at other paths.

## Rollout order

1. Run the disposable proof on a host with Docker access. Record the image digests and the observed results before building the production stack.
2. Start PostgreSQL for a fresh installation and verify the isolated databases. Prepare the seal key against that database, then start OpenBao.
3. Initialize the vault and publish the restricted credential. Run the start command again, then bring the stack down and back up. Observe that both services become healthy and that the test secret and credential persist.
4. Hand the credential contract and Compose service names to the sprint 2 plans after the standalone checks pass.

---
<!-- ⛔ implementation details and gate evidence below -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: Prove storage and restart behavior**

A disposable fixture validates the two upstream capabilities on the same OpenBao and PostgreSQL images the implementation will use. It also shows whether the retained key is what lets a replacement OpenBao process open the data.

**Done when**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1a | Writing a secret adds records to PostgreSQL. A replacement OpenBao container reads that same secret after PostgreSQL restarts. | `postgres_storage_round_trip` and `static_seal_restart`. |
| 1b | A replacement container exits without the key file. Restoring the original key reopens the same secret. The fixture is removed. | `missing_seal_key_blocks_unseal` and the final `cleanup=true` output. |

**Tasks**

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 1 | Run the throwaway PostgreSQL and static seal proof. | `scouting.md` under “Disposable proof for work item 1”. The nearest production target is `deploy/openbao.hcl` and its upstream `storage "postgresql"` and `seal "static"` blocks. The nearby Compose volume pattern is `design/openpencil/compose.yml`, `volumes.caddy-data`. |

**Work item 1. Run the disposable proof**

The proof has no retained implementation files. Use temporary `proof.sh`, `proof.test.sh` and `bao.hcl` in the directory created by the commands in [scouting.md](scouting.md#disposable-proof-for-work-item-1). Append the sanitized result to that document. The test writer owns `proof.test.sh`. It uses the disposable OpenBao health status, the `proof/sample` secret and the table `public.openbao_kv_store`.

Tests written first in temporary `proof.test.sh`:

- `postgres_storage_round_trip` writes a known value to the disposable mount. It asserts that the row count is greater than before the write and that a replacement OpenBao container reads that value with the same PostgreSQL volume.
- `static_seal_restart` asserts that the replacement container reports `initialized=true`, `sealed=false` and `storage_type=postgresql` within 60 seconds. The test never calls unseal.
- `missing_seal_key_blocks_unseal` removes the original key from the mounted path. It asserts that a fresh OpenBao process exits and the row count stays at least as high as after the write. It then restores the original key, starts a fresh container, and asserts that the value is readable again.

Start the red run with a stub that returns unchanged row counts, sealed or unavailable status and no recovered value through the test's fixture functions. Confirm that assertions report the values expected above. The item is risky because it tests a key and restart ordering. Mutate the storage backend to transient storage and expect `postgres_storage_round_trip` to fail. Point the seal configuration at a wrong key for the positive restart and expect `static_seal_restart` to fail. The acceptance observation is the four output lines that report row growth, unsealed state and value recovery, the missing-key result, and cleanup. Record the image digests alongside those lines.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An in-memory development vault could appear to start twice without using PostgreSQL. | High | Read rows from PostgreSQL and read the original value after replacing the OpenBao container. |
| A test might accidentally reuse an existing stack or keep its bootstrap material. | Medium | Give the fixture a unique project label and private directory. Trap process exits and check that fixture containers, network and volume are gone. |

**Verify.** The three proof assertions gate subsequent work. Run them on a Linux Docker host with access to the pinned images. No device check is needed.

**Rollout detail.** For step 1, run the commands in [scouting.md](scouting.md#disposable-proof-for-work-item-1), record observed output, and stop the build queue if either positive assertion fails.

**WS-2: Start PostgreSQL and open the vault inside the stack**

PostgreSQL is the durable store. OpenBao is a separate process with separate database credentials. A preparer can inspect PostgreSQL before deciding whether making a seal key is safe.

**Done when**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 2a | The two roles can connect to their own databases and are denied at the other database. The application database survives a container replacement. | `postgres_roles_are_isolated` and `postgres_volume_persists`. |
| 2b | PostgreSQL becomes healthy only when the two authenticated network queries pass. Neither daemon publishes a host port. | `postgres_waits_for_both_databases` and `vault_has_no_host_port`. |
| 2c | Preparing an existing installation preserves the key. A missing key with existing vault state prevents startup. | `seal_key_is_created_once` and `missing_key_preserves_existing_vault`. |

**Tasks**

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 2 | Create PostgreSQL, isolated database roles and their local credential files. | `deploy/compose.yaml`, `services.postgres` and `volumes.pgdata`, modeled on `design/openpencil/compose.yml`, `volumes.caddy-data`. `deploy/postgres-init.sql` follows the official image's `docker_process_init_files` contract. |
| 3 | Prepare the static seal and add the live OpenBao service. | `deploy/openbao.hcl`, `storage "postgresql"` and `seal "static"`, and `deploy/stack-prepare.sh`, `seal`. The upstream PostgreSQL storage and static seal anchors are in scouting. |

**Work item 2. Create the PostgreSQL service and its local credentials**

Create `deploy/compose.yaml`, `deploy/.gitignore`, `deploy/postgres-init.sql`, `deploy/stack-prepare.sh`, `deploy/test/vault-testlib.sh` and `deploy/test/postgres.sh`. Use `postgres:16.16-alpine`. Set the default Compose project name to `lgtm`. The `postgres` service uses `restart: unless-stopped`, volume `pgdata` at `/var/lib/postgresql/data` and only the internal Docker network `data`. The `stack-prepare` one-shot service uses the same image under the `setup` profile. It mounts the protected state root at `/state`. Its `database` subcommand checks state and creates the three database password files before PostgreSQL starts. Mount the named volume read-only in that helper to detect a pre-existing `PG_VERSION`.

The Compose file accepts an absolute `LGTM_STACK_STATE_DIR` prepared as a local directory with mode `0700`. Use `./.state` relative to the Compose file when the variable is absent. The launcher in item 6 resolves the same default to an absolute path. Use `deploy/.gitignore` to exclude that default. Reject symlinked state files and fail if a credential is missing or invalid after PostgreSQL has initialized. Create files on the same filesystem by writing a private temporary file and atomically publishing it. Generate independent 32-byte random values and encode each as 64 lowercase hexadecimal characters followed by one newline.

| Relative to the state directory | Owner and group by numeric identifier | Mode | Runtime reader |
|---|---|---|---|
| `postgres-admin.password` | 70 and 70 | `0400` | PostgreSQL setup, via `POSTGRES_PASSWORD_FILE`. |
| `postgres-app.password` | 70 and 10001 | `0440` | PostgreSQL bootstrap and health probes. Sprint 2 mounts this file alone for the application database login. |
| `postgres-bao.password` | 70 and 10002 | `0440` | PostgreSQL bootstrap and health probes. OpenBao gets a read-only mount of this file alone. |

Confirm the PostgreSQL image's numeric user and group in the pinned image at build time. Mount its three files under `/run/lgtm-db/` with their original basenames. Use the PostgreSQL interactive terminal `psql` to read the two role credentials from the mounted files during first-time initialization. Use Structured Query Language (SQL) with quoted parameters, and suppress password values in failures as well as normal output. The application login and database are both `lgtm_app`. The OpenBao login and database are both `lgtm_bao`. Neither login is a database administrator or can create other roles or databases. Set `POSTGRES_HOST_AUTH_METHOD=scram-sha-256` for network connections. Revoke public connect access from `lgtm_app`, `lgtm_bao`, `postgres` and `template1`, and grant each service login connect access to its database. The health check runs an authenticated `SELECT 1` through PostgreSQL's loopback network listener against each service database. The temporary initialization process listens only on a Unix socket. This prevents reporting healthy during that phase.

Tests written first in `deploy/test/postgres.sh`:

- `postgres_roles_are_isolated` reads the generated password files inside the fixture. It asserts that both roles can create and query a scratch table in their own database and that crossing into the other database is denied in both directions. It checks that neither role can create a database or login.
- `postgres_waits_for_both_databases` starts a fixture with the second database creation paused during first start. It asserts that PostgreSQL remains unready until both authenticated network queries pass. After disabling the second login in the disposable project, it asserts the health check fails again.
- `postgres_volume_persists` writes a sample row as `lgtm_app`, replaces the PostgreSQL container while retaining `pgdata`, and asserts that the row is present and none of the role credential files changed. It verifies that Compose declares no PostgreSQL host port.

Use a Compose fixture with an authenticated default role but without the target role grants, real authenticated readiness check or durable volume for the initial red assertions. Keep the initialization fixtures within the owning test file. The item is risky because it touches credential files and startup ordering. Grant public access to the OpenBao database and expect `postgres_roles_are_isolated` to fail. Replace the health check with a Unix-socket probe and expect `postgres_waits_for_both_databases` to fail. Acceptance is `bash deploy/test/postgres.sh` reporting the three test names with `PASS`.

**Work item 3. Prepare the seal key and start OpenBao**

Create `deploy/openbao.hcl` and `deploy/test/seal.sh`. Change `deploy/compose.yaml` and add the `seal` subcommand to `deploy/stack-prepare.sh`. Call `seal` after PostgreSQL is healthy. On authenticated query failure, stop without changing files. Query `lgtm_bao` for `public.openbao_kv_store` with `to_regclass`, and check for any row when the table exists. The data test is conservative. Any row, any existing private bootstrap journal or any published credential prevents creating a new key. Exit with `seal key missing for existing vault` in that case. A present key must be a regular 32-byte file with the required owner and mode. The helper keeps it byte-for-byte. A structurally valid but wrong key must still fail the OpenBao startup health check. This preparation step creates the empty `admin/` and `server/` mount directories with root ownership and mode `0700` if missing. It checks the ownership and mode on later runs.

The key is `bao/unseal.key` under the state directory, with mode `0400` and numeric user and group 10002. The OpenBao daemon runs as that user and group. Its read-only key path is `/run/lgtm-bao/unseal.key`. Use `seal "static"`, `current_key_id = "lgtm-1"` and a `file://` reference to that path. Use `storage "postgresql"` with table `openbao_kv_store`. Construct `BAO_PG_CONNECTION_URL` in the OpenBao process from its mounted password file, using host name `postgres`, port `5432` and database `lgtm_bao`. Keep that connection string out of rendered Compose settings and console output. Configure a single node. Set `max_connect_retries=0` so a daemon restart can wait for PostgreSQL. Set `disable_mlock = true` for the unprivileged container.

The `openbao` service uses `openbao/openbao:2.7.3` with `restart: unless-stopped`. Declare its Compose dependency on healthy PostgreSQL. It joins `data` and the internal network `vault`. Bind its Transmission Control Protocol (TCP) listener to its container interfaces on port `8200`, with Transport Layer Security (TLS) disabled on the internal network. Leave its port unpublished. Set `BAO_ADDR` in the OpenBao service to the loopback listener. Use `bao status` for the health check, which passes only with an initialized and unsealed vault. Make the status check silent. The OpenBao process can read only its database credential and seal key among the generated files.

Tests written first in `deploy/test/seal.sh`:

- `seal_key_is_created_once` runs `stack-prepare seal` twice against an empty vault database. It asserts regular-file type, length, mode and numeric ownership. It compares identity and content fingerprints across calls, and verifies a replacement OpenBao process accepts the original file.
- `missing_key_preserves_existing_vault` manually initializes a disposable OpenBao with real PostgreSQL and writes a fixture record. It stops OpenBao, moves the key into test staging, and runs the preparer. It asserts a nonzero exit and the exact missing-key diagnostic. It asserts that no key was generated and compares row count before and after. It restores the original key and confirms the value can be read.
- `vault_has_no_host_port` asserts that the runtime container has no host port mapping, including after a restart. It checks memberships in `data` and `vault`, and checks that a container on the intended network can reach the listener.

Stub the seal preparation to leave the target absent. For the missing-key red assertion, stub it to claim success when the key is absent. Use a published host port in a disposable Compose override to exercise the topology assertion. The item is risky because it touches key material and service ordering. Mutate the helper to generate a replacement when records exist and expect `missing_key_preserves_existing_vault` to fail. Change the key mode to `0644` and expect `seal_key_is_created_once` to fail. Acceptance is `bash deploy/test/seal.sh` reporting the three test names with `PASS`.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| An initialized database with a missing key could be paired with a new key. | High | Probe the existing database and local bootstrap state before writing the file. Keep the original key immutable during ordinary setup. |
| PostgreSQL can answer on a local socket before initialization finishes. | Medium | Gate on authenticated network queries to the two owned databases. |
| Both processes listen on container interfaces. | High | Leave their ports unpublished and give the vault and database only the intended internal network memberships. |

**Verify.** Run `bash deploy/test/postgres.sh` and `bash deploy/test/seal.sh` against a real PostgreSQL and OpenBao on Linux Docker. Run each mutation on a disposable fixture. No device check is needed.

**Rollout detail.** For step 2, the launcher completes `stack-prepare database`, waits for `postgres`, completes `stack-prepare seal`, and then starts `openbao`. The setup sequence and timeout are specified in item 6.

**WS-3: Hand off a restricted server credential**

The server needs an identity scoped to its signing key. A one-shot initialization job configures that identity. Steady-state runs keep using the existing seal key and credential file. The launch command checks both services before reporting success.

**Done when**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 3a | The protected file can obtain a 15-minute token that reads and writes the initial server path. Attempts at sibling, tool and administrative paths return 403. | `app_role_access_is_scoped` and `app_role_can_log_in_again`. |
| 3b | Bootstrap revokes the initial root token. A second run preserves the key and credential and makes no provisioning calls. | `bootstrap_revokes_root_before_publish` and `bootstrap_is_idempotent`. |
| 3c | A restart on the same state reads the existing vault record after both services become healthy. The command fails without replacing the key if the original file is lost. | `stack_reopens_existing_vault` and `stack_refuses_missing_key`. |

**Tasks**

| # | Task | Anchor (file + symbol) |
|---|---|---|
| 4 | Configure the server path, policy and AppRole against an initialized vault. | `deploy/openbao-provision.sh`, `lgtm-server` AppRole and `deploy/openbao-server-policy.hcl`, `lgtm/data/server/link/signing-key`. The nearest upstream contracts are the AppRole and policy references in scouting. |
| 5 | Initialize once, recover a partial setup, and publish the protected credential. | `deploy/openbao-init.sh`, `openbao-init` job. The nearest source is `floorplan/plan.json`, the `deploy/openbao-init.sh` entry. |
| 6 | Provide one command for the initial start and later restarts. | `deploy/up.sh`, entry point named in the `go-s2-stack-web` brief, and `deploy/test/stack-vault.sh`. `design/openpencil/compose.yml` documents invocation of a repository-local Compose setup. |

**Work item 4. Provision the server's OpenBao identity**

Create `deploy/openbao-tools.Dockerfile`, `deploy/openbao-server-policy.hcl`, `deploy/openbao-provision.sh` and `deploy/test/vault-policy.sh`. Build the helper image from the same pinned OpenBao image and install `curl` and `jq` for request and JavaScript Object Notation (JSON) handling. This is the only work item that writes those files. Test the provisioner against a disposable initialized vault with a test administrator. Use the fixture pattern from work item 1 so the test can run before item 2.

The provisioner reads the OpenBao location from `BAO_ADDR` and reads the temporary administrator token from `/state/admin/root.token`. It takes the chosen secret identifier from its protected staging file, which item 5 creates. Enable the version 2 key-value engine at mount `lgtm`. Enable AppRole at `auth/approle`, and configure role and policy `lgtm-server`. Bind a reusable secret identifier with `secret_id_ttl=0` and `secret_id_num_uses=0`. Use the AppRole custom secret identifier endpoint to register the staged value. Give the role `token_type=batch`, `token_ttl=15m`, `token_max_ttl=15m` and `token_no_default_policy=true`. The policy grants only `create`, `read` and `update` at `lgtm/data/server/link/signing-key`. Repeated provisioning during a partial bootstrap reuses the staged identifier. Look it up before writing it and distinguish an absent identifier from an authorization or transport error. Use the existing configuration if it matches. Exit on an unexpected configuration.

The provisioner writes the resulting `role_id` and `secret_id` object to a protected staging path given by item 5. Verify that the staged credentials can log in before returning success. Keep the login token in memory and pass secret values to request tooling through protected files or standard input. Keep them out of command arguments, Compose settings and logs.

Tests written first in `deploy/test/vault-policy.sh`:

- `app_role_access_is_scoped` logs in with a fixture credential and checks the response has `lease_duration=900`, a nonrenewable token and the intended policy. It asserts write, read and update at the exact signing-key path. It asserts 403 when reading a sibling server secret, reading or writing a tool secret, listing mount metadata, changing a policy or creating a token. Seed the denied paths using the test administrator where a secret needs to exist.
- `app_role_can_log_in_again` asserts that the same file can log in twice, yielding distinct token values. It checks that an incorrect secret identifier is denied. A second provisioning run must keep the same registered identifier and policy.

Start with a fixture role and a policy with empty capabilities so the positive assertion fails on 403. The item is risky because it grants access and creates credentials. Widen the policy to `lgtm/data/*` and expect `app_role_access_is_scoped` to fail. Set `secret_id_num_uses=1` and expect `app_role_can_log_in_again` to fail. Acceptance is `bash deploy/test/vault-policy.sh` reporting the two test names with `PASS`.

**Work item 5. Initialize once and publish the credential**

Create `deploy/openbao-init.sh` and `deploy/test/vault-init.sh`. Add `openbao-init` to `deploy/compose.yaml` as a one-shot service in the `setup` profile. It runs the helper image as root and joins only `vault`. Configure its `BAO_ADDR` with the HTTP scheme, service name `openbao` and port `8200`. It mounts the protected bootstrap directory at `/state/admin`, the credential output directory at `/state/server`, and both scripts and the policy read-only. The output directory is owned by root with mode `0700`. Docker mounts the credential file itself into the application container. Keep the seal key and database password out of this container's mounts. Its private state is `admin/` under the stack state directory, owned by root with mode `0700`. The daemon and the future server have no mount into that directory.

Wait up to 60 seconds for OpenBao's initialization status. On an uninitialized store with no setup record or credential file, call `PUT /v1/sys/init` with `recovery_shares=1` and `recovery_threshold=1`. The status query is `GET /v1/sys/init`. Save the response under `admin/init-response.json` in mode `0600`, using durable file publication. Save the first member of `recovery_keys_base64` in `admin/recovery.key`, mode `0600`. Save `root_token` to `admin/root.token`, mode `0600`. Once those files are durable, remove the raw response. Temporary and final material stays on the local state filesystem. If initialization committed before a usable response was saved and the vault is now initialized without trusted bootstrap material, stop with `vault is initialized but bootstrap state is incomplete`. Do not attempt another initialization or discard the database.

On an unfinished run with usable bootstrap material, resume the recorded step. Keep a random candidate secret identifier in `admin/server-secret-id.pending`, mode `0600`, and ask item 4's provisioner to write its JSON result to `admin/server-openbao.pending.json`. Confirm OpenBao accepts the staged login. Then call `POST /v1/auth/token/revoke-self` with the initial root token and confirm that `GET /v1/auth/token/lookup-self` returns 403. Persist `admin/root.revoked` with mode `0600` before removing the root token file. If an interruption follows revocation, verify the pending login and the revoked token or durable revocation marker before completing publication. Preserve the private recovery share and remove transient bootstrap credentials. Final publication uses an atomic rename of the validated credential on the same filesystem. A valid completed install returns `vault already configured` without a credential rewrite, a provisioning call or a new login. A missing published file with no resumable state fails closed with the incomplete-state diagnostic. No administrator material is printed.

Use the following credential contract for the sprint 2 server. The default host file is `deploy/.state/server/openbao.json`. Its relative location under `LGTM_STACK_STATE_DIR` is `server/openbao.json`. It is owned by numeric user and group 10001, with mode `0400`. Mount that file read-only at `/run/lgtm/openbao.json`. It contains one UTF-8 JSON object and a trailing newline. Require exactly two nonempty string fields, `role_id` and `secret_id`. Set `LGTM_OPENBAO_CREDENTIAL_FILE` to the mounted path. The application uses `LGTM_OPENBAO_ADDR`, built from the Hypertext Transfer Protocol (HTTP) scheme, service name `openbao` and port `8200`. Sign in by sending the two fields to `POST /v1/auth/approle/login`. Use `auth.client_token` in the `X-Vault-Token` header. Keep it in memory and log in again before the returned lease expires. The secret identifier in the file lasts until revoked. It can be replaced by an administrator using the retained recovery share after the vault is open. A single-file bind mount requires recreating the server container after an atomic replacement of that file.

The logical path convention is `<kind>/<owner>/<name>`. The signing key is `server/link/signing-key` under mount `lgtm`. Read and write at `/v1/lgtm/data/server/link/signing-key`. The stored object has field `private_seed_b64`, which contains the standard base64-encoded 32-byte Ed25519 private seed. Its first write uses the key-value engine's compare-and-set create-only option, `options.cas=0`. On a concurrent creation conflict the server reads the stored key. The server implementation belongs to sprint 2.

Tests written first in `deploy/test/vault-init.sh`:

- `bootstrap_revokes_root_before_publish` initializes a disposable vault with a test-only observer of the root token and publish boundary. It asserts that lookup using that token returns 403 before the credential appears. It checks the exact file mode, owner, group and JSON fields, and verifies that the credential can log in.
- `bootstrap_is_idempotent` records fingerprints and modification times for the credential and key. It runs the setup job twice and verifies the second run returns the already-configured message. It asserts identical files and uses a recording transport in front of real OpenBao to verify no provisioning request or login occurred during that second run.
- `bootstrap_resumes_after_root_revocation` interrupts a disposable run just after revocation and before publication. It asserts that a second run publishes the original staged identifiers and that only one credential is registered for the role. It also asserts that logs and container configuration lack the captured fixture credentials.
- `bootstrap_refuses_incomplete_state` starts with an initialized store and no published or recoverable setup state. It asserts a nonzero exit, the exact incomplete-state diagnostic and unchanged vault records.

For red tests, stub the bootstrap with a successful exit that leaves no credential and no diagnostic. Use disposable seeded state and an injected interruption at the request boundary for the resume test. The item is risky because it handles the initial root token and publishes the server credential. Remove token revocation and expect `bootstrap_revokes_root_before_publish` to fail. Replace the completed-state branch with provisioning and expect `bootstrap_is_idempotent` to fail. Acceptance is `bash deploy/test/vault-init.sh` reporting the four test names with `PASS`.

**Work item 6. Start and verify the whole sprint 1 stack with one command**

Create `deploy/up.sh`, `deploy/README.md` and `deploy/test/stack-vault.sh`. The command from the repository root is `bash deploy/up.sh`. Default to the Compose project `lgtm` and the resolved state directory `deploy/.state`. Honor `COMPOSE_PROJECT_NAME` and `LGTM_STACK_STATE_DIR` for isolated tests and installations. Support Linux Docker and a local filesystem with `flock`. Create the state root with mode `0700` and validate it before starting Compose. Bind each installation's state directory to one Compose project. Record the project name in the state directory and refuse a mismatch. Protect the full startup sequence with an exclusive lock in that root.

Run `docker compose -f deploy/compose.yaml run --rm --no-deps stack-prepare database`. Then run `docker compose -f deploy/compose.yaml up -d --wait --wait-timeout 60 postgres`. Run the same preparation job with `seal` and start OpenBao with `docker compose -f deploy/compose.yaml up -d openbao`. Run `docker compose -f deploy/compose.yaml run --build --rm --no-deps openbao-init`. Finish with `docker compose -f deploy/compose.yaml up -d --wait --wait-timeout 60 postgres openbao`. Exit nonzero as soon as a step fails. Print `postgres and openbao are healthy` after both health checks pass. State directories and the named volume persist through a plain Compose `down`.

The operator guide records startup, normal shutdown, the meaning of the missing-key diagnostic, and how to restore the original key with its original ownership and mode. Explain that initial root material is revoked and that the recovery share needs local protection. Give the smoke test command. Sprint 2 extends this launcher to start the server only after successful vault setup.

Tests written first in `deploy/test/stack-vault.sh`:

- `stack_reopens_existing_vault` runs the launcher in a fresh project. It asserts exact success output and the two Compose health states. It logs in using the credential file through a fixture container running as user 10001. It writes a fixture signing-key value. It performs ordinary Compose `down`, reruns the same command and asserts that the vault reports unsealed and that the old credential can read the original value. It compares the seal key and credential fingerprints without printing them.
- `stack_refuses_missing_key` stops the fixture after persisting data and moves the original key into the protected test directory. It reruns the launcher and asserts nonzero exit, the exact missing-key diagnostic, and no replacement key or running OpenBao container. It restores the key and verifies the same credential and value after restarting.
- `stack_bootstrap_is_serialized` overlaps two launcher invocations on one disposable state directory and records the phase boundaries. It asserts that setup phases do not interleave, and that both successful invocations end with one retained key and one server identity.

Use a launcher stub that exits successfully before performing setup for the red start and missing-key assertions. Use a stub with an intentionally overlapping setup phase to reach the serialization assertion. The item is risky because it controls startup and privileged job ordering. Remove the directory lock and expect `stack_bootstrap_is_serialized` to fail. Ignore the seal preparation exit and expect `stack_refuses_missing_key` to fail. Acceptance is `bash deploy/test/stack-vault.sh` reporting the three test names with `PASS`.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A partially completed setup could lose or keep privileged bootstrap credentials. | High | Store a private bootstrap record, verify revocation before publication, and exercise the interruption and incomplete-state cases. |
| The server's credential could read tool secrets or administer the vault. | High | Use an exact path policy and verify denied operations against real OpenBao. |
| A second start could replace the key or identity. | High | Serialize setup and check the original secret after stopping and starting the full sprint 1 stack. |

**Verify.** Run `bash deploy/test/vault-policy.sh`, `bash deploy/test/vault-init.sh` and `bash deploy/test/stack-vault.sh` on Linux Docker with real PostgreSQL and OpenBao. The isolated test projects use unique names and state paths, and their teardown removes only their own containers, volumes and state. The lead checks the exact commit and verifies the merged work after each item. The frozen tests define each item's acceptance check. The risky mutations must produce named assertion failures on temporary branches. No device or Go server is needed.

**Rollout detail.** For steps 3 and 4, `bash deploy/up.sh` prints `postgres and openbao are healthy` when ready. Use `docker compose -f deploy/compose.yaml down` to stop the default project. For a named installation, supply the same `COMPOSE_PROJECT_NAME` and `LGTM_STACK_STATE_DIR` as its launcher. Run the same launcher again. The standalone smoke test supplies an isolated project and checks both runs.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The source universe is D4, D9 and D10 of “Decisions of the owner”, the first-start and storage requirements of “Secrets live in OpenBao”, the secret name rule of “A vault on the server, and tools that use a key the agent never reads”, the PostgreSQL and OpenBao entries in “The compose stack”, the applicable clauses of rules R9 and R10, and the OpenBao restart clause of C13. The source is the approved foundation design. The work-item source is the Stack Vault brief's four work items and the task's first-proof and file handoff requirements. [Foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md), [bounded assignment](../../../.foundry/go-rewrite/tasks/plan-go-s1-stack-vault.md).

| Source item | Workstream | Status |
|---|---|---|
| D4, PostgreSQL from the start in Compose | WS-1, WS-2 | Items 1 and 2 prove and build the database. |
| D9, the server holds secrets in the vault | WS-3 | Items 4 and 5 provision access to the server's secret. |
| D10, OpenBao alongside PostgreSQL | WS-1, WS-2, WS-3 | Items 1 through 6 prove the backbone, configure it, and check the startup flow. |
| Secrets live in OpenBao, storage and automatic unsealing | WS-1, WS-2 | Items 1 and 3 prove and configure static unsealing and the dedicated database. |
| Secrets live in OpenBao, initial setup and restricted credential | WS-3 | Items 4 and 5 implement the identity and protected file. |
| The secret name rule of the vault and tools ruling | WS-3 | Items 4 and 5 fix the initial path and its data format. |
| The compose stack, vault and database entries | WS-2, WS-3 | Items 2, 3 and 6 define the named volume, topology and launch command. |
| R9, keep secrets out of output and user surfaces | WS-2, WS-3 | Items 2 through 5 keep credentials in protected files and exercise the privilege boundary. |
| R10, real PostgreSQL for database tests | WS-1, WS-2, WS-3 | Item 1 uses the pinned PostgreSQL image. Items 2 through 6 exercise the real database in their fixtures. |
| C13, OpenBao comes back unsealed after stack restart | WS-1, WS-3 | Items 1 and 6 prove unsealing after restart. The server's retrieval of the actual signing key is the other clause, owned by sprint 2. |
| Brief item 1 and the task's first-proof requirement | WS-1 | Item 1 is a prerequisite of the implementation. |
| Brief items 2 and 3 | WS-2 | Items 2 and 3 scope the database and configure OpenBao. |
| Brief item 4 and the task's credential handoff requirement | WS-2, WS-3 | Items 3 through 6 split key preparation, credential provisioning, first start and restart verification. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, “What this project is”, places the product parts behind the shared contract. | The stack provides infrastructure and a configuration file. The server's internal vault adapter owns the application integration. |
| `AGENTS.md`, “Where things are”, keeps new work out of the reference engine, and the epic assigns `deploy/` to Stack. | The production paths are in `deploy/`. The first proof lives in a temporary directory. |
| `AGENTS.md`, habit 1, credits copied code under `third_party/` and in `NOTICE`. | The plan uses upstream component interfaces and new setup scripts. Code copying would need the repository's credit process. |
| `AGENTS.md`, habit 2, records problems in the issue log. | This bounded assignment records the Docker access limitation in scouting and the result file. The orchestrator owns edits outside the plan and result. |
| `AGENTS.md`, habit 3, sends finished work to the base branch through a pull request. | The build lead verifies the exact commit. The sprint uses the epic's pull request handoff. |
| `AGENTS.md`, writing style, keeps private details out of tracked plans. | Run the repository checker and inspect the untracked deliverables too. Keep secret material out of command transcripts. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on | Can run alongside | Design statements |
|---|---|---|---|---|---|---|---|---|
| 1. Disposable proof | `infra` | `stack/infra` | medium | The proof crosses two real services and has positive and negative restart cases. | No | None. | None within this plan. | D4, D10, R9, R10, C13. |
| 2. PostgreSQL service | `infra` | `stack/infra` | medium | One database process must initialize and check two isolated roles and databases. | Yes | Item 1. | Item 4. | D4, D10, R9, R10. |
| 3. OpenBao service and seal | `infra` | `stack/infra` | medium | The preparer consumes an existing database state check and one static seal format. | Yes | Items 1 and 2. | Item 4. | D4, D10, R9, R10, C13. |
| 4. Server identity policy | `infra` | `stack/infra` | medium | The provisioner can be tested on its own against an already initialized vault. | Yes | Item 1. | Items 2 and 3. | D9, D10, R9, R10. |
| 5. First-start job | `infra` | `stack/infra` | medium | This item handles initialization and publication using the policy provisioner as a fixed dependency. | Yes | Items 3 and 4. | None within this plan. | D9, D10, R9, R10. |
| 6. Launch and restart | `infra` | `stack/infra` | medium | The launcher sequences existing checks and reuses the same daemon and setup commands. | No | Items 2, 3 and 5. | None within this plan. | D4, D10, R9, R10, C13. |

Load. Six first-test runs, six build runs and six mutation verification runs give 18 baseline seats before plan review. Under the epic's alternating assignment, allow six test and build seats per primary vendor, then three mutation seats per vendor if they alternate. For each full plan review round, budget four seats from the other primary vendor, one from the planner's vendor, and one at the additional logic seat. The orchestrator checks remaining weekly limits before dispatch and allocates seats by the epic. No remaining balances are visible to this assignment.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| Handoff for adversarial review | Pending. | The bounded planner assignment ends after self-check. The orchestrator records findings in this fold after review. |

</details>
