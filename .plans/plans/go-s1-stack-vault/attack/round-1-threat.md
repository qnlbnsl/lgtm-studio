# Round 1, seat `threat`: review of the plan `go-s1-stack-vault`

Task identifier: review-go-s1-stack-vault-r1-threat. Nonce: b0c811. Model and effort: claude-sonnet-5-5, high.

## Verdict

REVISE

## Top finding

The plan revokes the bootstrap root token and then relies on the retained recovery share to replace the server credential and to extend the policy, but on the pinned OpenBao image the recovery share cannot mint a new root token without an authenticated token, so after sprint 1 nobody can ever administer the vault again except by wiping it.

## Findings

### threat-r1 1 — MAJOR — D9

No administrator path exists after root revocation, and the sentence that promises one is false on the pinned image.

- Evidence: `plan.md:24` revokes the bootstrap administrator's token. `plan.md:204` keeps only the "private recovery share". `plan.md:206` says the server's secret identifier "can be replaced by an administrator using the retained recovery share after the vault is open". `plan.md:187` makes the provisioner "exit on an unexpected configuration", and `plan.md:204` makes a completed install return `vault already configured` with no provisioning call. `docs/specs/2026-11-25-go-rewrite-foundation-design.md:430` says tool secrets "fit beside" the signing key later, and line 483 says harness sign-ins go into the same vault.
- Observed on `openbao/openbao:2.7.3` (digest `sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9`) with PostgreSQL storage and a static seal, initialised with one recovery share: `bao operator generate-root -init` with no token returns 403 `permission denied`, both before and after the root token is revoked. With the root token it succeeds. The recovery share alone therefore does not produce a new root token on this version. The plan creates no other administrative policy or token.
- Failure scenario: sprint 3 or a later wave needs the server to read a second path, or the owner needs to revoke the server's secret identifier because `.state/server/openbao.json` leaked from a backup. The secret identifier has `secret_id_ttl=0` and `secret_id_num_uses=0` (`plan.md:187`), so it never expires. The policy file cannot be changed, because the init job skips and no token can write a policy. The credential cannot be revoked or replaced. The only way out is to delete the `lgtm_bao` database and initialise again, which destroys every secret stored by then.
- Fix: Before the root token is revoked, the init job creates a policy `lgtm-admin` and an AppRole `lgtm-admin`. The policy covers `sys/policies/acl/*`, `auth/approle/*`, `sys/generate-root/*` and the `lgtm/` mount. Its credential is published only under `admin/` (root, mode `0700`). Add a work item that gives `deploy/` one administrator command. The command logs in with that AppRole, re-applies `openbao-server-policy.hcl` when the file changed, and rotates the server credential. Rotation registers a new secret identifier, writes the new credential file by atomic rename, and destroys the old identifier by its accessor. Replace "exit on an unexpected configuration" with "update a policy or role that differs from the file, and refuse anything else". Add a test `admin_can_rotate_and_widen_policy_after_root_revocation`. Extend work item 1 so the disposable proof also initialises over the HTTP API, revokes the root token and attempts the generate-root path. That proof would have caught the false sentence. Delete the sentence at `plan.md:206` unless the new test passes.

### threat-r1 2 — MAJOR — D10

A crash between the OpenBao initialisation commit and the first durable write of the response leaves the stack permanently unstartable, and the operator guide gives no repair.

- Evidence: `plan.md:202` stops with `vault is initialized but bootstrap state is incomplete` and says "Do not attempt another initialization or discard the database". `plan.md:225` lists what the operator guide covers, and the incomplete-state diagnostic is not on the list. `bootstrap_refuses_incomplete_state` (`plan.md:215`) proves only that the refusal is clean.
- Failure scenario: `docker compose run openbao-init` is killed, or the machine loses power, after `PUT /v1/sys/init` returned and before `admin/init-response.json` is durable. The vault holds nothing yet, but `bash deploy/up.sh` now exits non-zero on every run. The plan forbids the only fix, which is to empty `lgtm_bao`, and does not describe how to do it. The operator must work out a destructive procedure alone, next to the vault that holds the seal key.
- Fix: Document in `deploy/README.md` the one repair for the empty-vault case. Stop `openbao`, empty the `lgtm_bao` database with the `lgtm_bao` login, keep `bao/unseal.key`, and rerun the launcher. Provide it as a subcommand, `bash deploy/up.sh --reset-empty-vault`. The subcommand refuses unless `public.openbao_kv_store` holds only the rows that initialisation writes, and it prints what it will remove. Add a test `incomplete_bootstrap_repair_reaches_healthy`. Narrow the window as well: write the response straight to the final private file with `fsync` before any other step.

### threat-r1 3 — MAJOR — C13

The case where the state directory outlives the PostgreSQL volume is not specified, and the standard Compose reset creates it.

- Evidence: `plan.md:223` says "State directories and the named volume persist through a plain Compose `down`", which implies `down -v` removes the volume and keeps `deploy/.state`. `plan.md:204` defines only three outcomes: fresh install, resumable run, and completed install returning `vault already configured`. `plan.md:137` makes `seal` keep an existing key and tests only the missing-key direction. State files under `bao/`, `admin/` and `server/` are root-owned with mode `0700` (`plan.md:137`, `plan.md:200`), so an operator without root cannot remove them with `rm -rf`.
- Failure scenario: the operator runs `docker compose -f deploy/compose.yaml down -v` to start over. `pgdata` is gone, but `.state/server/openbao.json`, `.state/admin/root.revoked` and `.state/bao/unseal.key` remain. The store is uninitialised and a published credential exists. The init job matches none of its three cases, or it matches "completed install" and returns `vault already configured` without looking at the vault. `up.sh` then waits 60 seconds for health that cannot come. Removing `.state` by hand fails on root-owned directories.
- Fix: Add the fourth state to the init job's table: the store is uninitialised and the state directory says configured. The job stops with a named diagnostic, `state directory is ahead of the database`. Provide `bash deploy/up.sh --reset`, which runs a one-shot container to remove the state directory contents and `down -v` together after an explicit `--yes`. Make "vault already configured" require a successful `GET /v1/sys/init` returning `initialized=true`. Add a test `stack_refuses_state_ahead_of_database` and a second that proves `--reset` returns the project to a fresh install.

### threat-r1 4 — MAJOR — R9

The PostgreSQL health check is not authenticated, so the stated property of the stack is false.

- Evidence: `plan.md:125` and `plan.md:102` (`2b`) say the health check runs an "authenticated `SELECT 1` through PostgreSQL's loopback network listener". The plan sets only `POSTGRES_HOST_AUTH_METHOD=scram-sha-256`. Observed on `postgres:16.16-alpine` (digest `sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d`) with exactly that setting: the effective `pg_hba.conf` is `local all all trust`, the two loopback `host` lines, both `trust`, the same three lines for `replication`, and then `host all all all scram-sha-256`. A a `psql` call over the loopback address with no password returned `1`. From the container's own bridge address the same call asked for a password.
- Failure scenario: `postgres-bao.password` is replaced, for example by restoring a state directory from another installation, so it no longer matches the role. The loopback check still reports healthy. OpenBao then fails to connect with a password error after `up.sh` has already declared PostgreSQL ready. Separately, any process in the PostgreSQL container, such as a shell opened for debugging, is database superuser without a password.
- Fix: Set `POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 --auth-local=scram-sha-256`, so initdb writes no `trust` lines. Make the health check pass `PGPASSWORD` read from the mounted file. Add a test `postgres_has_no_trust_rules` that reads `pg_hba.conf`, and a test that changes one role password in the fixture and expects the health check to fail. The mutation at `plan.md:133` ("replace the health check with a Unix-socket probe") should become "revert the initdb arguments".

### threat-r1 5 — MINOR — plan

The plan does not say who owns the two databases, and on PostgreSQL 16 the owner decides whether OpenBao can create its table.

- Evidence: `plan.md:125` creates the databases and grants connect, with no `OWNER`. Observed on `postgres:16.16-alpine`: a login that is not the database owner got `ERROR: permission denied for schema public` on `create table public.t`. OpenBao creates `public.openbao_kv_store` itself (`plan.md:73`, `plan.md:137`).
- Failure scenario: the builder creates `lgtm_bao` owned by `postgres`. Work item 3 fails at first start, and the builder repairs it by granting `CREATE` on `public` or by making the role a superuser, which undoes the isolation of work item 2.
- Fix: State `CREATE DATABASE lgtm_bao OWNER lgtm_bao` and `CREATE DATABASE lgtm_app OWNER lgtm_app`. Add to `postgres_roles_are_isolated` that an owner cannot connect to the other database even after `REVOKE`, and that `lgtm_app` cannot read `public.openbao_kv_store` through any grant.

### threat-r1 6 — MINOR — D10

Images are pinned by mutable tag, the helper image installs unpinned packages, and the digests the plan promises to record are never used.

- Evidence: `plan.md:115` and `plan.md:141` use `postgres:16.16-alpine` and `openbao/openbao:2.7.3`. `plan.md:185` builds a helper image from the OpenBao image and installs `curl` and `jq` at build time. `docs/` decision row 7 and `plan.md:44` say to "record the resolved digests", and `scouting.md:55` has no output yet. The tags are published and valid, and the digests I observed are in finding 1 and finding 4.
- Failure scenario: a rebuilt `16.16-alpine` or a retagged image changes what runs next to the vault. `docker compose run --build openbao-init` downloads whatever `curl` and `jq` the Alpine mirror serves, in the container that holds the root token.
- Fix: In `deploy/compose.yaml` use `image: postgres:16.16-alpine@sha256:<digest>` and `openbao/openbao:2.7.3@sha256:<digest>` and `FROM` the same digest in `deploy/openbao-tools.Dockerfile`. Pin the two packages by version in the Dockerfile. Put the digests in `scouting.md` when item 1 is run. Add a test `images_are_pinned_by_digest` that fails when a service image has no digest.

### threat-r1 7 — MINOR — D10

The plan does not say that the networks are `internal: true`, and it does not reduce what a container on them can do to the plain-HTTP traffic that carries the root token.

- Evidence: `plan.md:115` and `plan.md:141` call the networks "internal" but never state `internal: true`. `plan.md:141` disables TLS on `8200`. The root token, the recovery share and the AppRole secret identifier all cross that bridge in plain text (`plan.md:202`, `plan.md:206`). The server joins `data` and `vault` in sprint 2.
- Failure scenario: without `internal: true`, PostgreSQL and OpenBao can open outbound connections. A compromised or misbehaving service has an exit. Docker's default capability set includes `NET_RAW`, so a container on the same bridge can forge ARP replies and read another container's plain-HTTP traffic.
- Fix: Declare `internal: true` on `data` and `vault`. Add `cap_drop: [ALL]` to every service of the stack and add back only what each needs, and set `security_opt: [no-new-privileges:true]`. Add a test `stack_services_have_no_egress_and_no_net_raw` that runs a probe container on each network. The design accepts HTTP inside the stack, so this finding does not ask for TLS.

### threat-r1 8 — MINOR — R9

The build context of the helper image is not named, and `deploy/.state` holds the seal key and the root token.

- Evidence: `plan.md:185` creates `deploy/openbao-tools.Dockerfile`. `plan.md:223` runs `docker compose run --build` on every start. The plan never sets `build.context`. The default state directory is `deploy/.state` (`plan.md:117`), under the folder that a context of `deploy/` would send.
- Failure scenario: a builder sets the context to `deploy/`. Docker sends `.state` to the build daemon, and a later `COPY . /` puts the unseal key and `admin/root.token` into an image layer. In the other direction, the root-owned `0700` directories make the build fail for an operator without root.
- Fix: State in the plan `build.context: ./openbao-tools` with a Dockerfile that has no `COPY`, and add `deploy/.dockerignore` that lists `.state`. Add a test that builds the helper image and asserts its layers contain no file from `.state`.

### threat-r1 9 — MINOR — D10

The resume rule has no table of crash points, and the tests exercise one of them.

- Evidence: `plan.md:204` says "resume the recorded step" and names five files (`init-response.json`, `recovery.key`, `root.token`, `server-secret-id.pending`, `server-openbao.pending.json`) and `root.revoked`. `plan.md:214` tests only the interruption just after root revocation.
- Failure scenario: the job is killed after `server-secret-id.pending` is written and before the provisioner registers the identifier, or after the identifier is registered and before `server-openbao.pending.json` is written. Which case the second run takes is not written down, and each depends on the lookup distinguishing an absent identifier from an error. My probe of `secret-id/lookup` for an absent identifier returned a response my filter did not match, so the shape of "absent" has to be pinned by a test, not assumed.
- Fix: Add a table to the plan with one row per crash point, listing the files present, what the second run does, and the test that proves it. Add a test that kills the init job at each row, with an injected failure at each boundary, and expects one registered identifier and one published credential at the end. Add the exact response (status and body) of the lookup for an absent identifier to the provisioner's description.

## Answers to the attack questions

Lens file:

1. Trust boundaries: the inputs are the state directory (files on disk), the compose environment variables `LGTM_STACK_STATE_DIR` and `COMPOSE_PROJECT_NAME`, the OpenBao HTTP API on the internal networks, and the PostgreSQL role logins. The plan checks the state root (mode, symlinks, project binding), file type, size, owner and mode of every credential file, and the `public.openbao_kv_store` row count. Gaps: the PostgreSQL loopback is `trust` (finding 4), and the state directory ahead of the database is unchecked (finding 3).
2. Inventory: PostgreSQL administrator, application and vault passwords (made by `stack-prepare database`, mode `0400` or `0440`, read through `POSTGRES_PASSWORD_FILE` and mounted files, never logged by plan). Seal key `bao/unseal.key` (made once, mode `0400`, mounted into OpenBao only). Root token, recovery share and init response (`admin/`, mode `0600`, root-only directory). Server `role_id` and `secret_id` (`server/openbao.json`, `0400`, owner 10001). The connection URL (built inside the OpenBao process, kept out of rendered Compose output). Nothing is passed on a command line or logged by design. The leak routes I could not rule out without code are in the post-build checks.
3. How each credential ends: the PostgreSQL passwords and seal key have no rotation and no end, which the owner's one-machine decision allows. The root token ends at revocation (finding 1 concerns what replaces it). The server secret identifier never expires, and the plan's replacement route does not work (finding 1). The login token lasts 15 minutes and is not renewable, which I observed (`lease_duration=900`, `renewable=False`, batch).
4. Encryption: the design accepts plain HTTP and PostgreSQL without TLS inside the stack. At rest, the vault is encrypted under `bao/unseal.key` and the key is a separate file from the PostgreSQL volume, as the design requires. The gap is that the networks are not declared internal and keep `NET_RAW` (finding 7). Issue 1633 concerns the link and the web proxy, and this plan touches neither, so nothing here depends on it.
5. Dependencies: both images are pinned by tag only and the helper image adds unpinned packages (finding 6). The plan trusts the OpenBao and PostgreSQL maintainers and the Alpine mirror.
6. Untrusted parties: another user on the same machine is stopped by the state root at mode `0700`, by `0400` and `0440` modes, and by mounting only the file each container needs. The website and machines do not touch this plan. A process in the PostgreSQL container is unauthenticated superuser today (finding 4). The server container, once built, holds the secret identifier and can reach OpenBao for its one path only, which I verified: sibling path, metadata list and token lookup all return 403.
7. Post-build checks: listed below.

Task file:

- a. Facts run? The planner did not run them, because Docker access was denied, and `scouting.md:55` records no output. I ran the proof script from `scouting.md:60` to `scouting.md:188` unchanged, with `postgres:16.16-alpine` and `openbao/openbao:2.7.3`. It exited 0. Rows in `public.openbao_kv_store` went from 28 to 31 after a write. A fresh OpenBao container reported initialised, unsealed and `storage_type=postgresql`, and read the value back after PostgreSQL restarted. With the key moved away OpenBao exited, and with it restored the value was readable again. Cleanup left no container, volume or network. Both facts hold. The image tags exist. The plan follows from these observations, but it goes further than the proof: the proof uses a loopback listener, the host's own user and a PostgreSQL superuser login, whereas production uses the wildcard address on port 8200, user 10002 and a non-owner role, and the proof never initialises over HTTP, never revokes the root token and never tests an administrator route. That is finding 1 and finding 5. The tags, commands and output still have to be recorded in `scouting.md` by the builder of item 1, and the digests I observed are in findings 1 and 4.
- b. A second run of the init job on a completed install returns `vault already configured` without a rewrite, a provisioning call or a login (`plan.md:204`), but it does so without asking the vault (finding 3). A run after a half-finished run resumes from files, and only one crash point is tested (finding 9); a crash before the first durable write has no repair (finding 2). The credential: default path `deploy/.state/server/openbao.json`, relative path `server/openbao.json` under `LGTM_STACK_STATE_DIR`, one UTF-8 JSON object with exactly `role_id` and `secret_id` and a trailing newline, owner and group 10001, mode `0400`, in a root-owned `0700` directory, mounted read-only at `/run/lgtm/openbao.json` (`plan.md:206`). What replaces it: the plan says an administrator with the recovery share, which does not work (finding 1).
- c. Each image is pinned by tag, not by digest (finding 6). No port is published by either service, so the loopback question does not arise in this plan. Compose declares no host port, and `vault_has_no_host_port` tests it. The later `web` service must bind loopback only. Health checks exist for both services, and the order is `postgres` healthy, then `seal`, then `openbao`, then `openbao-init`, then both healthy (`plan.md:223`), though the PostgreSQL check is not authenticated (finding 4). PostgreSQL holds two databases with two separate roles, `lgtm_app` and `lgtm_bao`, plus the `postgres` administrator, and connect is revoked from public (`plan.md:125`). The plan omits the owner (finding 5).

## Post-build checks

- PB-1: A wrong database password makes OpenBao fail to connect. Expected: neither the OpenBao log, the `docker compose logs` output nor `docker inspect` of any container contains the password or the connection URL.
- PB-2: Run the whole of `bash deploy/up.sh` with `docker compose config` captured. Expected: the rendered configuration, the console and the logs contain none of the three database passwords, the unseal key bytes, the root token, the recovery share, or the `secret_id`.
- PB-3: Read `/proc/<pid>/environ` of the OpenBao process as another user of the container. Expected: the connection URL is not readable by any other uid (the process runs as 10002 and nothing else shares that uid).
- PB-4: After first start, list the owner, group and mode of every file under the state directory and in `docker inspect` mounts. Expected: they match the tables at `plan.md:119` and `plan.md:206`, and no container other than `openbao-init` mounts `admin/`.
- PB-5: Kill the init job at every row of the crash-point table (finding 9). Expected: a rerun ends with one registered secret identifier and one published credential, or the named diagnostic.
- PB-6: Run `docker compose up -d` while `openbao` is uninitialised and an unrelated container on `data` calls `PUT /v1/sys/init`. Expected: record the result. Initialisation by a party other than the init job would give that party the root token. The window is closed in practice, because only the init job runs on `vault` before the server exists, but sprint 2 must not add the server to the stack before initialisation completes.
- PB-7: Stop the whole stack, copy `pgdata` alone, and start a fresh stack with that copy and no `bao/unseal.key`. Expected: OpenBao refuses to open and no secret is readable, which is the design's claim at `docs/specs/2026-11-25-go-rewrite-foundation-design.md:227`.
- PB-8: Restart OpenBao with `disable_mlock = true` on a host with swap enabled. Expected: record whether the vault's memory can reach swap, and note it as the accepted cost of an unprivileged container.

## Grade

6 out of 10. Counts: 0 blockers, 4 majors, 5 minors.
