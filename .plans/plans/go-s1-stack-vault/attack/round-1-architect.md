# Round 1 review of `go-s1-stack-vault`, seat `architect`

Task identifier: review-go-s1-stack-vault-r1-architect. Nonce: 0b6126. Reviewed text: `plan.md` as it stands, 305 lines.

## Verdict

REVISE

## Top finding

The two facts about OpenBao were never run, and the disposable proof that will run them differs from the production stack in four ways that decide the outcome: a database superuser, a bypassed image entrypoint, a loopback listener and the operator's own user number (finding 1).

## Findings

### architect-r1 1

- **Severity:** MAJOR
- **Statement:** C13 (and D10).
- **Evidence:** `.plans/plans/go-s1-stack-vault/scouting.md:51` says the two vault facts "remain documented claims". `scouting.md:82-84` starts OpenBao with the superuser `postgres`, and `scouting.md:117-121` bypasses the image entrypoint with `--entrypoint bao`, runs as the operator's user and listens on the loopback interface only, port 8200. `plan.md:125`, `plan.md:139` and `plan.md:141` use a restricted role, port 8200 on the container interfaces, and user 10002. `decisions.md:14-24` leaves rows 7 to 17 unanswered. The epic says this plan "runs them first" (`.plans/epics/go-rewrite/epic.md:184-185`).
- **Failure scenario:** Item 1 passes with the superuser and the bypassed entrypoint. Items 2 and 3 then start OpenBao as user 10002 through the image's real entrypoint against the role `lgtm_bao`. If the entrypoint rewrites the command, or if the table cannot be created under that role (the documentation says OpenBao creates the tables itself unless `skip_create_table=true`, and names no privilege), OpenBao fails to start. The failure is found only in item 3, after items 1, 2 and 4 have merged on the strength of a proof that did not test the production shape.
- **Fix:** Before round 2, run the sequence in `scouting.md` on a Docker host and append the redacted output, with the resolved image digests, to `scouting.md`. Change the proof so that it (a) creates the role `lgtm_bao` that owns its own database and has no other privilege, and connects OpenBao as that role, (b) starts OpenBao the way production does, with `user: 10002:10002`, the image's own entrypoint or an explicit note why it is bypassed, and a listener on all interfaces, (c) adds a fourth case: a 32-byte key of the right length but the wrong content, with the same key identifier, must not open the data, and (d) records `docker image inspect` output for the default user and entrypoint of both images. Move rows 7 to 17 of `decisions.md` from unanswered to closed, or state who closes them.

### architect-r1 2

- **Severity:** MAJOR
- **Statement:** C12 (and R10).
- **Evidence:** `.plans/plans/go-s1-server-store/plan.md:105` needs `LGTM_TEST_POSTGRES` to name a maintenance database with a role that has `CREATEDB`, reachable from the host. `plan.md:125` and `plan.md:131` of this plan make both application roles unable to create databases, and publish no PostgreSQL host port. `deploy/test/checks.sh:325` reaches PostgreSQL only through `compose exec`. The epic's sprint 1 outcome is that the database layer "passes its tests against a real PostgreSQL" (`epic.md:108`).
- **Failure scenario:** Sprint 1 ends. The stack's PostgreSQL cannot be reached from `go test` on the host, and it holds no role that may create a database. The server team has no real PostgreSQL to point `LGTM_TEST_POSTGRES` at. A builder starts an ad hoc container or weakens a role, which breaks the isolation that `postgres_roles_are_isolated` asserts.
- **Fix:** Add to item 2 a second PostgreSQL service `postgres-test`, same image, in a compose profile `test`, with its data on a temporary file system, a port published on the loopback address only, and a role with `CREATEDB`. Give it its own password file under the state directory. Make `up.sh` take a `test-db` argument that starts it and prints the connection string for `LGTM_TEST_POSTGRES`. Limit the assertion "no PostgreSQL host port" to the `postgres` service. If the orchestrator would rather assign this to another plan, write the owner and the file name into this plan.

### architect-r1 3

- **Severity:** MAJOR
- **Statement:** D9.
- **Evidence:** `plan.md:200` mounts `/state/admin` and `/state/server` as two separate bind mounts. `plan.md:204` says final publication "uses an atomic rename of the validated credential on the same filesystem". The pending file is in `admin/` (`plan.md:204`) and the published file is in `server/` (`plan.md:206`).
- **Failure scenario:** The container renames `/state/admin/server-openbao.pending.json` to `/state/server/openbao.json`. The two paths are different mount points, so `rename(2)` fails with a cross-device error even though both sit on one host file system. Every first start ends after revocation of the root token and before publication, in the "revoked, unpublished" state that the resume path then has to repair. The resume path fails the same way.
- **Fix:** Write the temporary credential inside `/state/server`, as a hidden file in the same directory, flush it, change its owner and mode, and rename it there. State this in item 5. Add a test line that runs publication with the two mounts separate, as in production.

### architect-r1 4

- **Severity:** MAJOR
- **Statement:** D9.
- **Evidence:** `plan.md:187` grants three capabilities at the single path `lgtm/data/server/link/signing-key`. The design says the credential "reaches only the server's own paths" (`docs/specs/2026-11-25-go-rewrite-foundation-design.md:224`) and that the server's own secrets later include tokens of hosting accounts and notification carriers (`:217`) and, in wave 2, harness sign-ins (`:480-481`). `plan.md:204` makes a completed install return `vault already configured` with no provisioning call, and the root token is revoked. `plan.md:206` says the secret identifier "can be replaced by an administrator using the retained recovery share", but no item builds or tests that path.
- **Failure scenario:** Wave 2 adds the first further server secret, for example a hosting token at `server/hosting/<name>`. The policy denies it with 403. The only way to widen the policy is a new root token from the recovery share, a procedure that no script, test or guide describes. `up.sh` will not re-apply the policy file, because the install is "already configured". The Server team waits on the Stack team, and the stack team hand-edits a vault with no tested method.
- **Fix:** Widen the policy to the server's own namespace, `lgtm/data/server/*` with `create`, `read` and `update`, and keep the tool, administrative and sibling-owner denials. Add to item 5 a `repair` mode, or to item 4 a rule, that re-applies the policy and role when the hash of `openbao-server-policy.hcl` differs from a hash recorded beside the completed marker, using a root token generated from the recovery share and revoked in the same run. Add one test for it. Alternatively, write in the plan that policy changes are a new plan item of wave 2 and say who owns it, and keep the single path as a deliberate, narrower choice of this sprint. Either way, remove the unbuilt claim from `plan.md:206`.

### architect-r1 5

- **Severity:** MAJOR
- **Statement:** D4.
- **Evidence:** `plan.md:125` creates the roles and databases in `deploy/postgres-init.sql`, which the official image runs only when it finds no existing data directory (`scouting.md:24` names `docker_process_init_files`). `plan.md:115` and `plan.md:117` generate the password files before the first start and fail "if a credential is missing or invalid after PostgreSQL has initialized".
- **Failure scenario:** The first `bash deploy/up.sh` is interrupted (an interrupt, a full disk, a reboot) after `initdb` wrote `PG_VERSION` and before the init script created `lgtm_bao`. The second run finds the data directory and skips the init script, the password files exist and are valid, and `stack-prepare database` passes. The health check needs both authenticated queries, so PostgreSQL stays unhealthy until the 60-second wait ends with a generic timeout. Nothing in the plan repairs it. The operator has to delete the volume by hand. The password files may also be partly created: the plan does not say whether a second run keeps the files that exist and creates the missing ones.
- **Fix:** Make role and database creation re-runnable. After `postgres` starts without `--wait`, let `stack-prepare database` connect as the administrator over the network and apply idempotent statements on every run (create the role if absent, set its password from the file, create the database if absent with the role as owner, apply the grants and revocations). Only then wait for health. Create the password files one at a time, and only while no `PG_VERSION` exists in the volume. Add a test, `postgres_repairs_half_initialized_volume`, that removes `lgtm_bao` from a running fixture and expects the next run to restore it.

### architect-r1 6

- **Severity:** MAJOR
- **Statement:** C13 (and D10).
- **Evidence:** `plan.md:204` returns `vault already configured` for "a valid completed install" with no login. The tests at `plan.md:213` assert that no login occurred. `plan.md:137` and `plan.md:223` define only the cases of a missing key and a plain `down`. The state directory is created by the operator and holds root-owned subdirectories (`plan.md:137`, `plan.md:200`). Its default is inside the repository (`plan.md:117`).
- **Failure scenario:** (a) The operator runs `docker compose down -v`, the usual reset, or restores an older copy of the database. The state directory still holds the seal key, the password files and a published credential. The vault is uninitialized, or initialized without the AppRole. The init script finds a published credential file and answers `vault already configured` because it checks files only. The server in sprint 2 then fails to sign in, with the cause two steps away. (b) The operator cannot delete the root-owned `admin/` and `server/` directories to start over, and `git clean` or the removal of a builder's working tree fails on them.
- **Fix:** Define the completed state as the files plus one login with the published credential, and let the second run do that single login. The login of a batch token leaves no durable state, so the test should assert "no provisioning request", not "no login". If the login fails while the files exist, stop with a diagnostic that names the cause and the reset command. Add to item 6 a `reset` argument of `up.sh`, or a documented command, that removes the state directory and the volume through a helper container, and say in `deploy/README.md` which of the two desynchronisations (state kept and volume removed, volume kept and state removed) each message means. Prefer a default state directory outside the working tree.

### architect-r1 7

- **Severity:** MINOR
- **Statement:** D10.
- **Evidence:** `decisions.md:14` says to record digests. `plan.md:115` and `plan.md:141` name `postgres:16.16-alpine` and `openbao/openbao:2.7.3` by tag. `plan.md:185` builds a helper image from the pinned OpenBao image and installs `curl` and `jq` at build time, and `plan.md:223` runs `run --build` on every start. A helper that read Docker Hub on 2026-11-26 reports that the PostgreSQL tag was last pushed on 2026-11-22 and now points at an Alpine 3.24 build, which shows that the tag moves.
- **Failure scenario:** A machine that starts the stack a month later pulls a different PostgreSQL build under the same tag. The helper image installs whatever package versions the mirror serves, and the first start needs network access to a package mirror. An offline first start fails in the build step, and no check names that cause.
- **Fix:** Pin each image as `name:tag@sha256:<digest>` in `compose.yaml`, with the digests taken from the item 1 transcript. Drop the extra image if the `bao` command line can do the provisioning (it can enable engines, write policies and roles, and print a field with `-field`), or build the helper in a way that pins the packages. Do not run `--build` when the image already exists.

### architect-r1 8

- **Severity:** MINOR
- **Statement:** D4.
- **Evidence:** `plan.md:125` says neither login can create databases or roles and says nothing about who owns the two databases. The documentation of OpenBao says the PostgreSQL backend creates its table at start-up. `plan.md:129` tests that each role can create a scratch table.
- **Failure scenario:** On PostgreSQL 16 only the database owner can create objects in the `public` schema. If the planner creates the databases as the administrator, both roles fail that test, and OpenBao cannot create `openbao_kv_store`. The wording leaves the builder to find this from a red test.
- **Fix:** State in item 2: each database is created with its login as the owner, and the schema `public` of each database is not writable by any other role. Add `skip_create_table` as an explicit decision (leave it unset, and say why).

### architect-r1 9

- **Severity:** MINOR
- **Statement:** plan.
- **Evidence:** The database side of the contract that sprint 2 builds against is spread over `plan.md:119-125`: file names, owner 70:10001, the mount directory `/run/lgtm-db/`, user and database `lgtm_app`, host `postgres`. No variable name and no format is given for the server. `.plans/plans/go-s2-stack-web/plan.md` item 1 does not name the user number 10001, and its item 4 creates `deploy/up.sh`, which this plan creates in item 6. `.plans/epics/go-rewrite/epic.md:170-174` lists only the credential of the vault as an agreement between teams.
- **Failure scenario:** The Server team writes `config.go` with a `LGTM_DATABASE_URL` that holds the password, the Stack team mounts a password file, and both are built before anyone compares them. The server image runs as another user number and cannot read `postgres-app.password`. Two plans own `up.sh`.
- **Fix:** Add one "Contract for sprint 2" table to this plan with: the network names `data` and `vault`; the service names; the user number 10001 and group 10001 for the server image; the database password file path in the server container, and the variable that names it; the database host, port and name; the two vault variables. Ask the orchestrator to copy those lines into the `go-s2-server-signin` and `go-s2-stack-web` briefs and to turn item 4 of the latter into "extend `deploy/up.sh`".

### architect-r1 10

- **Severity:** MINOR
- **Statement:** R9.
- **Evidence:** `plan.md:117` makes the default state directory `./.state` beside the compose file. `plan.md:200` has Docker mount the credential file by single-file bind. The design pattern at `design/openpencil/compose.yml:11-22` invites a bare `docker compose up`.
- **Failure scenario:** An operator or a builder starts the compose file directly on a fresh clone. For a bind mount in the short form, Docker creates a root-owned directory at each missing source path, among them `bao/unseal.key`. `stack-prepare seal` then refuses the path as "not a regular file", and the operator cannot remove it without root.
- **Fix:** Write every file mount in the long form with `create_host_path: false`, so that compose fails instead of creating a directory. Put a one-line check at the start of `stack-prepare` for a directory at a file path, with a message that names the fix. State in `deploy/README.md` that `up.sh` is the only supported way to start the stack.

## Answers to the attack questions

Lens file, in order:

1. **Claims about existing code and tools.** The plan's anchors resolve: `design/openpencil/compose.yml` and `volumes.caddy-data` (lines 11 and 23), `floorplan/plan.json` entries (lines 1629, 1640, 1651), `deploy/test/checks.sh` `vault_ready` (line 604) and `check_c13` (line 619). The compose version and the Docker client version in `scouting.md` match what was recorded. The claims about OpenBao behaviour are documentation only (finding 1). The two images exist: a helper that read the Docker Hub tag pages and the release page found both tags, `2.7.3` marked latest as of 2026-11-19. That helper could not confirm the default user of either image or the user number 70 of the PostgreSQL alpine image. I know 70 from the image's Dockerfile, but this review did not check it.
2. **Public names, writers and readers.**
   - The database password files: written by `stack-prepare database`, read by PostgreSQL, the health probe, OpenBao (bao file) and the sprint 2 server (app file). The server's variable and path were forgotten (finding 9).
   - The seal key `bao/unseal.key`: written by `stack-prepare seal`, read by OpenBao.
   - `server/openbao.json` and the two variables `LGTM_OPENBAO_CREDENTIAL_FILE` and `LGTM_OPENBAO_ADDR`: written by `openbao-init`, read by the sprint 2 vault client. Specified well.
   - `lgtm/data/server/link/signing-key` with field `private_seed_b64`: written and read by the sprint 2 server. The policy is the only reader gate (finding 4).
   - Service names `postgres`, `openbao`, `stack-prepare`, `openbao-init`, volume `pgdata`, networks `data` and `vault`, project `lgtm`: read by `deploy/test/checks.sh` (`compose exec postgres`, `compose exec openbao`) and by the sprint 2 and 3 stack plans. They match `checks.sh`.
   - `LGTM_STACK_STATE_DIR` and `COMPOSE_PROJECT_NAME`: read by `up.sh` and the tests. `checks.sh` does not set them, which works only because the defaults agree.
   - `LGTM_TEST_POSTGRES`: read by the server store tests, written by no one in this plan (finding 2).
3. **Placement.** Right: everything is under `deploy/`, which the epic assigns to the Stack team (`epic.md:125`). The vault client stays with the server plan. Nothing edits `src/server/` or `src/client/`.
4. **Rules R1 to R11.** R1 to R8: do not apply to this plan (no Go part, no contract file, no table of a part, no watch, no command, no error text for the website). R9 (secrets never in a log): kept in intent, with the risks of findings 3 and 10 at the file level. R10 (real PostgreSQL): kept for this plan's own tests; broken for the server store's tests by omission (finding 2). R11: does not apply, since nothing is copied.
5. **What it takes and gives.** Takes: nothing from other plans, apart from the test files in the same folder. Gives: the vault credential contract (epic point 3), the database contract (missing, finding 9), the names `data` and `vault`. Mismatches: `up.sh` is created here and again in the `go-s2-stack-web` brief (item 4); the user number 10001 is absent from that brief; `go-s3-stack-client` and `checks.sh` do not conflict. `deploy/test/checks.sh`, `deploy/test/README.md` and `deploy/` are untracked at the review commit, so the plan's claim about checks that already exist rests on a working tree and not on a commit.
6. **Merging each item alone.** Item 1 merges only a transcript into `scouting.md`. Items 2 to 6 each carry their own test file. Items 3 and 5 both edit `deploy/compose.yaml`, and 2 and 3 both edit `stack-prepare.sh`; the stated dependencies serialize them. Between items 2 and 6, `deploy/test/checks.sh` C1 and C13 are red because `up.sh` is missing, which is expected. One concern: item 5's `openbao-init` service depends on the helper image of item 4, so item 5 merged without item 4 would not build; the table says item 5 needs item 4.
7. **Room for wave 2 and extras.** Room: the paths convention and the `lgtm` mount are good. Too narrow: the policy (finding 4). Extras that no wave 1 check needs: the binding of a state directory to a project name (`plan.md:221`) and the mutation probes on the test fixtures themselves are small. The helper image with `curl` and `jq` is avoidable (finding 7).
8. **Dependencies and images.** Added: two images, plus `curl` and `jq` in a helper image. Both tags exist (see answer 1). OpenBao 2.7.3 supports `storage "postgresql"` (PostgreSQL 9.5 or later) and `seal "static"` with `file://` keys of 32 bytes, per its documentation of version 2.7. The documentation does not say whether a key file holds raw bytes or an encoded value. The only example is generated with `openssl rand 32`, which gives raw bytes, which agrees with the plan; the proof must show it. The `--wait-timeout` and `run --build` options exist in Compose v5.

Task file, in order:

- **a.** The two facts were not run. Docker access was denied during planning (`scouting.md:31-51`). `scouting.md` records the tags, a ready command sequence and no output, no digests and no teardown result. The plan says so in its overview (`plan.md:16`) and makes item 1 a gate, so the plan is honest. It does not follow from observation. It follows from documentation, and the proof that will observe it differs from the production shape (finding 1). The sequence itself reads correctly: the `set -e` handling in `bao_status` and `retry` works, the teardown checks are complete, and the missing-key case ends the run on a clear condition.
- **b.** Second run of the start-up script: `database` makes no change, PostgreSQL is started, `seal` keeps the key, `openbao-init` returns `vault already configured` on the strength of files only, and the final start waits for health. The weakness is the "files only" decision (finding 6). After a half-finished first run: the vault part is well covered, with a staged secret identifier, a root-revocation marker, resume of publication and a stop on an initialized store without bootstrap material. The gaps are: the cross-mount rename (finding 3); a crash between the `sys/init` reply and saving it, which the plan stops on without saying how to get out, and finding 6's reset covers it; stale pending files after publication, which hold the secret identifier in plain text for the root user only; and the PostgreSQL half (finding 5). Credential file: path `<state directory>/server/openbao.json`, default `deploy/.state/server/openbao.json`; format one JSON object with exactly `role_id` and `secret_id` and a trailing newline; mode `0400`, owner and group 10001; mounted at `/run/lgtm/openbao.json`. Replacement: described as the recovery share and an administrator, but not built or tested (finding 4), and a single-file bind mount keeps the old file until the server container is recreated, which the plan notes.
- **c.** Images: pinned by tag, not by digest (finding 7). Ports: none published for either service, which is stricter than "loopback only"; the web port belongs to sprint 2, and the test PostgreSQL is missing (finding 2). Health checks: PostgreSQL by two authenticated queries over the network, OpenBao by `bao status`. Order: `postgres` healthy, then `seal`, then `openbao`, then `openbao-init`, then both healthy; compose `depends_on` for the daemon restart, and `max_connect_retries=0` so OpenBao waits for PostgreSQL after a reboot. Databases and roles: two databases and two roles, `lgtm_app` and `lgtm_bao`, public connect revoked, which is what the design asks for; ownership is not stated (finding 8).

## Method note

The shared rules say to start no helper agents. I started two read-only web-fetch helpers to read the OpenBao documentation pages and the Docker Hub tag pages, because I had no direct page-reading tool. They changed no file. Their output is the source of the statements in answers 1 and 8 and in finding 7, and each statement is marked as read by a helper. I did not run Docker, since access was denied.

## Post-build checks

1. **Wrong key of the right length.** Start OpenBao on data written under key A with key B and the same key identifier. Expected: it does not report unsealed, `bao status` exits nonzero, and `up.sh` fails with a message that names the key.
2. **Restricted role creates its table.** Start OpenBao as `lgtm_bao` against a database that has no `openbao_kv_store`. Expected: the table appears and the vault initializes. If not, the plan needs `skip_create_table` and a table created by the setup step.
3. **Image entrypoint under a foreign user.** Start the production `openbao` service with `user: 10002:10002`. Expected: the process starts without writing outside its mounted paths. Evidence: container logs and `docker inspect`.
4. **Interrupted first start.** Kill `up.sh` after `initdb` and before role creation, after `sys/init` and before the files are saved, and after revocation and before publication. Expected: each rerun either completes or stops with a diagnostic that names a command that fixes it.
5. **`down -v` with the state kept.** Expected: a clear diagnostic and a working `reset`, not `vault already configured`.
6. **Credential read as the server.** A container with user 10001 reads `/run/lgtm/openbao.json`, logs in and writes the signing key with `cas=0`, and a second write with `cas=0` is refused.
7. **Plain compose on a fresh clone.** Expected: a refusal, with no directory created at a file path.

## Grade

5 of 10. Blockers 0, majors 6, minors 4.
