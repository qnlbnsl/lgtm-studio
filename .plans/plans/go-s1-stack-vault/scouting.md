# Sources and checks for the PostgreSQL and OpenBao stack

## What exists at the starting commit

The source scan used commit `677fdca9`. The working copy of the approved design matched the file on `master`.

| File and anchor | What it establishes |
|---|---|
| [The foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#decisions-of-the-owner), Decisions of the owner, and [Secrets live in OpenBao](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#secrets-live-in-openbao) | D4, D9 and D10 give the database, vault and first-start boundary. The vault section requires its own database and automatic unsealing from a file. |
| [The foundation design](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#the-compose-stack), The compose stack, rules 9 and 10, and check 13 | The stack gives the vault an internal network connection. Tests use real PostgreSQL. A stack restart must reopen the initialized vault. |
| [The epic](../../epics/go-rewrite/epic.md#the-teams-and-what-each-may-edit), Stack team and plans table | The Stack team owns `deploy/`. The stack's credential file crosses into the server sign-in plan. |
| [The build loop](../../../docs/specs/2026-11-05-build-loop-design.md#4-build), Build subitems | Builders receive frozen tests that failed on their assertions. A risky item also requires a mutation run before the exact-commit check is accepted. |
| [The issue log](../../../docs/issues.md#1632-what-the-design-of-the-go-rewrite-leaves-open), issues 1632 through 1635 | Issue 1633 belongs to the link and web proxy. Issue 1634 belongs to link test entry points. Issue 1635 explains the use of three sizes. The stack work here uses a real database and has no link or server dependency. |
| [The floorplan](../../../floorplan/plan.json), `floorplan/plan.json` and its entries for `deploy/openbao.hcl`, `deploy/openbao-init.sh` and `deploy/compose.yaml` at lines 1629, 1640 and 1651 | These are planned file names. There was no `deploy/` or `src/server/` directory in the checkout during the scan. |
| [The existing Compose file](../../../design/openpencil/compose.yml), `services.openpencil` and `volumes.caddy-data` at lines 11 and 23 | This is the nearby pattern for explicit image tags, bind mounts and named volumes. Its service and port declarations belong to the design tool. |
| [The sign-in brief](../go-s2-server-signin/plan.md#work-items), work item 8, and [the web stack brief](../go-s2-stack-web/plan.md#work-items), work items 3 and 4 | The former consumes the vault credential. The latter supplies the running server, runtime mounts and the full-stack command. |
| [The private-data check](../../../scripts/check-private.mjs), `check` and `files` at lines 50 and 72 | The check examines tracked files. Newly written scouting and result files need an additional read. |

## Original source links for the first plan

| Target paths | Nearest pattern or upstream contract |
|---|---|
| `deploy/compose.yaml`, `deploy/.gitignore` and `deploy/test/vault-testlib.sh` | `design/openpencil/compose.yml` uses a named project, `services` and `volumes`. Compose defines dependency conditions and named internal networks. The tests use a unique project name and state directory for each run. [Compose workload specification](https://docs.docker.com/reference/compose-file/workload-spec/). |
| `deploy/postgres-init.sql`, `deploy/stack-prepare.sh` and `deploy/test/postgres.sh` | The PostgreSQL image starts an initialization server before its normal server and runs `docker_process_init_files` against a fresh data directory. The implementation needs to wait for real authenticated network queries. [Official PostgreSQL image entrypoint](https://github.com/docker-library/postgres/blob/master/docker-entrypoint.sh). |
| `deploy/openbao.hcl`, `deploy/test/seal.sh` and the `seal` subcommand of `deploy/stack-prepare.sh` | OpenBao's `storage "postgresql"` uses `BAO_PG_CONNECTION_URL` and defaults to `openbao_kv_store`. Its `seal "static"` supports a raw 32-byte key at a `file://` path. [PostgreSQL backend](https://openbao.org/docs/configuration/storage/postgresql/), [static seal](https://openbao.org/docs/configuration/seal/static/). |
| `deploy/openbao-tools.Dockerfile`, `deploy/openbao-server-policy.hcl`, `deploy/openbao-provision.sh` and `deploy/test/vault-policy.sh` | The nearest image and configuration references are the published OpenBao image and the AppRole login and policy contracts. [OpenBao image tags](https://hub.docker.com/r/openbao/openbao/tags), [AppRole](https://openbao.org/docs/auth/approle/), [policies](https://openbao.org/docs/concepts/policies/). |
| `deploy/openbao-init.sh`, `deploy/test/vault-init.sh`, `deploy/up.sh`, `deploy/README.md` and `deploy/test/stack-vault.sh` | The `openbao-init` job follows the floorplan's vault setup entry. The next stack brief's `up.sh` defines the local startup entry points. OpenBao documents its initial root token and how to revoke it after setup. Docker Compose defines `up --wait`. The pinned OpenBao client defines `InitRequest` and `InitResponse`. [OpenBao policies](https://openbao.org/docs/concepts/policies/#built-in-policies), [Compose up](https://docs.docker.com/reference/cli/docker/compose/up/), [OpenBao 2.7.3 initialization client](https://github.com/openbao/openbao/blob/v2.7.3/api/sys_init.go). |

The concurrent user-flow script `deploy/test/checks.sh` appeared after that scan. Its `vault_ready` function reads `bao status -format=json` without a custom address. Its `check_c13` function also checks the server and website. This plan uses the adjacent `deploy/test/stack-vault.sh` for the vault half of C13.

## Docker availability on 2026-11-26

The proposed image tags are `openbao/openbao:2.7.3` and `postgres:16.16-alpine`. The official sources list both tags. [OpenBao release](https://github.com/openbao/openbao/releases/tag/v2.7.3), [PostgreSQL image catalog](https://github.com/docker-library/official-images/blob/master/library/postgres).

```text
$ docker version
Client: Docker Engine - Community
 Version: 29.7.2
permission denied while trying to connect to the docker API at unix:///var/run/docker.sock

$ docker compose version
Docker Compose version v5.5.0

$ docker info --format '{{.ServerVersion}}'
permission denied while trying to connect to the docker API at unix:///var/run/docker.sock

$ podman info --format '{{.Host.OCIRuntime.Name}}'
Failed to obtain podman configuration: mkdir /run/user/1000/libpod: read-only file system
```

The Docker checks did not start a container, pull an image, or create a network or volume. The two vault facts remain documented claims. Work item 1 records the runtime output and teardown evidence before the stack uses those claims.

## Disposable proof for work item 1

The following is the proposed command sequence for a Docker-enabled host. It was prepared on 2026-11-26 and has no runtime output yet. The test writer first puts the observable assertions for storage, restart and the missing-key control in a script in this temporary directory. The builder makes those assertions pass with this setup. Bring only the redacted transcript back to this file.

The fixture uses the Docker command-line interface and Python 3 for JavaScript Object Notation (JSON) assertions. The temporary project uses a private Docker network. The PostgreSQL password and test value below belong only to this disposable fixture. Use a non-root Linux operator with Docker access. The Docker workload runs OpenBao as that operator's numeric user and group.

```bash
set -euo pipefail
umask 077
scratch=$(mktemp -d "${TMPDIR:-/tmp}/lgtm-bao-proof.XXXXXXXX")
project="lgtm-bao-proof-${scratch##*.}"
pg_image=postgres:16.16-alpine
bao_image=openbao/openbao:2.7.3

cleanup() {
  docker rm -f "$project-bao" "$project-pg" >/dev/null 2>&1 || true
  docker volume rm "$project-pg" >/dev/null 2>&1 || true
  docker network rm "$project" >/dev/null 2>&1 || true
  rm -rf -- "$scratch"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker pull "$pg_image"
docker pull "$bao_image"
docker image inspect --format '{{json .RepoDigests}}' "$pg_image" "$bao_image"
docker network create --internal "$project"
docker volume create "$project-pg"
docker run -d --name "$project-pg" --network "$project" --network-alias proof-pg \
  -e POSTGRES_PASSWORD=disposable-proof -e POSTGRES_DB=proof \
  --mount "type=volume,source=$project-pg,target=/var/lib/postgresql/data" "$pg_image"

retry() {
  for attempt in $(seq 1 60); do
    if "$@"; then return 0; fi
    sleep 1
  done
  return 1
}
retry docker exec -e PGPASSWORD=disposable-proof "$project-pg" \
  psql -h 127.0.0.1 -U postgres -d proof -Atqc 'SELECT 1'

head -c 32 /dev/urandom > "$scratch/unseal.key"
chmod 0400 "$scratch/unseal.key"
cat > "$scratch/bao.hcl" <<'HCL'
disable_mlock = true
storage "postgresql" {
  table = "openbao_kv_store"
}
seal "static" {
  current_key_id = "proof-1"
  current_key = "file:///proof/unseal.key"
}
listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = true
}
HCL

start_bao() {
  local connection
  connection=$(printf '%s://%s:%s@%s:%s/%s?sslmode=disable' \
    postgres postgres disposable-proof proof-pg 5432 proof)
  docker run -d --name "$project-bao" --network "$project" \
    --user "$(id -u):$(id -g)" -e BAO_ADDR=http://127.0.0.1:8200 \
    -e "BAO_PG_CONNECTION_URL=$connection" \
    --mount "type=bind,source=$scratch,target=/proof,readonly" \
    --entrypoint bao "$bao_image" server -config=/proof/bao.hcl
}
bao_status() {
  docker exec "$project-bao" bao status -format=json > "$scratch/status.json"
  local status=$?
  [ "$status" -eq 0 ] || [ "$status" -eq 2 ]
}
with_token() {
  docker exec -i "$project-bao" sh -c \
    'IFS= read -r BAO_TOKEN && export BAO_TOKEN && exec bao "$@"' sh "$@" \
    < "$scratch/token"
}
row_count() {
  docker exec "$project-pg" psql -U postgres -d proof -Atqc \
    'SELECT count(*) FROM public.openbao_kv_store'
}
assert_unsealed() {
  bao_status && python3 -c \
    'import json, sys; s=json.load(open(sys.argv[1])); assert s["initialized"] and not s["sealed"] and s["storage_type"]=="postgresql"' \
    "$scratch/status.json"
}

start_bao
retry bao_status
docker exec "$project-bao" bao operator init -recovery-shares=1 -recovery-threshold=1 \
  -format=json > "$scratch/init.json"
python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))["root_token"])' \
  "$scratch/init.json" > "$scratch/token"
retry assert_unsealed
with_token secrets enable -path=proof kv-v2
before=$(row_count)
with_token kv put -mount=proof sample value=postgres-proof >/dev/null
after=$(row_count)
test "$after" -gt "$before"
printf 'postgres_rows_before=%s postgres_rows_after=%s\n' "$before" "$after"

docker rm -f "$project-bao" >/dev/null
docker restart "$project-pg" >/dev/null
retry docker exec -e PGPASSWORD=disposable-proof "$project-pg" \
  psql -h 127.0.0.1 -U postgres -d proof -Atqc 'SELECT 1'
start_bao
retry assert_unsealed
test "$(with_token kv get -mount=proof -field=value sample)" = postgres-proof
printf 'initialized=true sealed=false storage=postgresql value_restored=true\n'

docker rm -f "$project-bao" >/dev/null
mv "$scratch/unseal.key" "$scratch/unseal.saved"
start_bao
for attempt in $(seq 1 20); do
  if assert_unsealed 2>/dev/null; then exit 1; fi
  if [ "$(docker inspect --format '{{.State.Status}}' "$project-bao")" = exited ]; then break; fi
  sleep 1
done
test "$(docker inspect --format '{{.State.Status}}' "$project-bao")" = exited
test "$(row_count)" -ge "$after"
printf 'missing_key_blocks_unseal=true\n'

docker rm -f "$project-bao" >/dev/null
mv "$scratch/unseal.saved" "$scratch/unseal.key"
start_bao
retry assert_unsealed
test "$(with_token kv get -mount=proof -field=value sample)" = postgres-proof
cleanup
trap - EXIT
test -z "$(docker ps -aq --filter "name=$project-")"
test -z "$(docker volume ls -q --filter "name=$project-pg")"
test -z "$(docker network ls -q --filter "name=$project")"
printf 'cleanup=true\n'
```

The proof passes when PostgreSQL shows more vault records after the write, the replacement OpenBao process reports unsealed and reads the exact test value, and the missing-key run fails to open the same data. A final rerun with the original key must recover that value. Record the tags, resolved digests, command outputs and teardown checks. Exclude the initialization response, the root token and the key bytes from the transcript.

## Plan self-check on 2026-11-26

The planner checked the plan format, the decision rows, eight acceptance observations, 13 traceability rows, the risks and verification notes for three workstreams, six work-item anchors, six guardrail entries and six execution sizes. The plan changes no screen. The plan keeps the epic's linked row and active status. The scope checker returned `ok: stack/infra`. A Bash syntax check of the proposed proof sequence exited 0.

The bounded assignment reserves adversarial review for the orchestrator. The attack log is ready for its findings.

The Node.js private-data check's default Git child process failed with `EPERM` while still returning Git exit 0 and its complete output. A minimal `spawnSync` call reproduced the error with a piped standard input. The same call succeeded with ignored standard input. The unchanged repository checker ran with Node.js 26.13.0 and a temporary preload that set only the unused input of its Git child to ignore. The tracked-file run exited 0. A second run included the scouting and result files in the input list without staging them. The planner also read the authored text for private details.

## Review round 1 evidence on 2026-11-26

The review reports refer to the baseline snapshot at `attack/plan-v1.md`. The command block above remains the historical run that two reviewers reproduced. The current planner checked the Docker daemon again with `docker info --format '{{.ServerVersion}}'` and got a permission denial at the local Docker socket. The current planner ran no container.

The critic and threat reports independently record the baseline proof passing and leaving no disposable resources. The logic report also tested an owning service role and a replacement key. These are reviewer observations. The narrower production-shape replay is still pending. [Critic report, task answer a](attack/round-1-critic.md#answers-to-the-attack-questions), [threat report, task answer a](attack/round-1-threat.md#answers-to-the-attack-questions), [logic report, What I ran](attack/round-1-logic.md#what-i-ran).

| Image name | Observed registry digest |
|---|---|
| `postgres:16.16-alpine` | `sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d` |
| `openbao/openbao:2.7.3` | `sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9` |

The baseline proof produced these redacted result lines and exited 0.

```text
postgres_rows_before=28 postgres_rows_after=31
initialized=true sealed=false storage=postgresql value_restored=true
missing_key_blocks_unseal=true
cleanup=true
```

The separate logic run reported PostgreSQL numeric user and group 70, and an OpenBao default command containing development mode. With a missing key OpenBao exited. A different 32-byte key left the same data sealed and restoring the original reopened it. The critic reported that an ownerless service role could not create the storage table and that ownership repaired startup. The threat report reproduced an unauthenticated root-generation refusal on the pinned image. These observations are tied to the cited reports.

| Current source or upstream anchor | What it establishes |
|---|---|
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md:215`, Secrets live in OpenBao, and lines 341 to 355 and 370 to 390 | The vault uses PostgreSQL and a local file, serves the stack only, and grants the server its own paths. The restart and real PostgreSQL checks remain separate. |
| `deploy/test/checks.sh`, `check_c1`, `run_check`, `vault_ready` and `check_c13` | The script calls an executable launcher and checks OpenBao status. Its full C13 expects a signing-key fingerprint from the eventual server. The retained sprint 1 tests use the surrounding test output style. |
| `deploy/test/README.md`, `LGTM_TEST_POSTGRES`, and `.plans/plans/go-s1-server-store/plan.md`, `NewDatabase` | The test database must be real, host reachable and allow database creation. The closest patterns for new `deploy/test/vault-testlib.sh`, `deploy/test/with-postgres.sh` and `deploy/test/vault-fixtures.sh` are `deploy/test/checks.sh` fixture names and its README. |
| `deploy/openbao-start.sh` and `deploy/openbao.hcl`, proposed entrypoint and storage settings | The pinned image's default command starts development mode. The planned wrapper specifies the production command and reads the password file. The nearest configuration pattern is the published storage backend. [PostgreSQL backend](https://openbao.org/docs/configuration/storage/postgresql/). |
| `deploy/postgres-init.sql`, `deploy/stack-prepare.sh`, `prepare-db` and `reconcile-db` | The Docker entrypoint only applies initialization hooks on an empty cluster. PostgreSQL chooses the first authentication rule. The repair job uses the existing administrator connection. [PostgreSQL image initialization](https://github.com/docker-library/docs/blob/master/postgres/README.md#initialization-scripts), [PostgreSQL 16 host authentication](https://www.postgresql.org/docs/16/auth-pg-hba-conf.html). |
| `deploy/openbao-bootstrap.sh`, `deploy/test/vault-bootstrap.sh`, and `deploy/openbao-init.sh`, `bootstrap.state` and `server/openbao.json` | The initialization and health reads answer different questions. The local publication rename must stay on one mount. [OpenBao health status](https://openbao.org/docs/api/system/health/), [OpenBao initialization client](https://github.com/openbao/openbao/blob/v2.7.3/api/sys_init.go), [Linux rename errors](https://man7.org/linux/man-pages/man2/rename.2.html#ERRORS). |
| `deploy/vault-admin.sh`, `deploy/openbao-admin.sh`, `deploy/openbao-admin-policy.hcl`, and `deploy/test/vault-admin.sh` | The proposed local identity uses named policy and AppRole endpoints. The older unauthenticated root endpoints are disabled by default. The exact absent-identifier response is to be captured by the pinned fixture before a builder implements the classification. [AppRole API](https://openbao.org/docs/api/auth/approle/), [Legacy root endpoint](https://openbao.org/docs/api/system/generate-root/), [Authenticated root endpoint](https://openbao.org/docs/api/system/generate-root-token/). |
| `deploy/openbao-tools/Dockerfile`, `deploy/openbao-tools/.dockerignore` and `deploy/.dockerignore` | The proposed context is separate from state. Bind sources use the long form, which can forbid automatic directory creation. [Docker Compose container specification](https://docs.docker.com/reference/compose-file/container-spec/). |

## Production-shape proof for work item 1

Reuse the historical script's temporary directory, unique container names, PostgreSQL row count check, guarded exit trap and cleanup assertions. Apply the following fixture changes before running it. The build check records the actual commands and sanitized output. The planner or orchestrator attaches that receipt here.

1. Pull both tag-and-digest references from the table above. Run `docker image inspect --format '{{.Os}}/{{.Architecture}} {{json .RepoDigests}} {{.Config.User}} {{json .Config.Entrypoint}} {{json .Config.Cmd}}'` for both. Record the platform and defaults separately from the wrapper override. Confirm PostgreSQL's effective user and group as 70.
2. In the disposable PostgreSQL cluster, create `lgtm_bao` with login, its fixture password, and neither superuser, database creation nor role creation privilege. Create database `lgtm_bao` owned by that login. Revoke public connection to it and the other ordinary databases. Use the production host and local password settings. Connect OpenBao with `lgtm_bao` rather than `postgres`. Query the storage row count as the service owner. Check the table begins absent and OpenBao creates it.
3. Adapt `start_bao` to run as `10002:10002`. Use a temporary copy of the production wrapper contract, `/bin/sh /openbao-start.sh server`, and supply the mounted password file. Mount configuration at `/openbao/config/lgtm.hcl` and the raw key inside a disposable mounted key directory at `/run/lgtm-bao/unseal.key`. Set the fixture key owner and group to 10002 with mode `0400` using a short-lived root container. Use `sslmode=disable`, the production listener on the container interfaces, and an internal network without host ports. The wrapper explicitly bypasses the image's default entrypoint to load the file credential and avoid the development command.
4. From the pinned OpenBao image on the fixture network, run `bao operator init -format=json -recovery-shares=1 -recovery-threshold=1`. Save the response in the private temporary directory. Require initialized, unsealed PostgreSQL status, then assert the same secret round-trip and restart as the historical proof. Keep fixture tokens and response bodies off the console.
5. Recreate the process with the key file removed from the mounted directory. Observe its diagnostic and absence of health. Replace it with a different correctly owned 32-byte file and verify the absence of health again without expecting an exit. Verify the existing PostgreSQL row count in each case. Restore the original key and assert the original sample value.
6. Stop and remove the disposable containers, volume, network and temporary files. Emit the four assertion results, image observations, the row counts and `cleanup=true`. A single failed assertion or incomplete teardown stops the implementation queue.

Use the pinned OpenBao image for the command-line client and host `jq` for structured results. Production-image verification and clean-host startup remain build checks. The production Compose file and wrappers are still planned.

## Round 2 sources and runtime limits

The reviewed plan snapshot is `attack/plan-v2.md`. It matched the working plan before the fold. The three available reports are [architecture](attack/round-2-architect.md), [logic](attack/round-2-logic.md) and [threat](attack/round-2-threat.md). The approved design's “Secrets live in OpenBao” section sets the storage, static seal, service boundary and restricted runtime credential. The epic's “The teams, and what each may edit” assigns `deploy/` to Stack and caps the plan at nine work items. Its sprint schedule places C12 in sprint 1, the full restart check in sprint 2 and C5 in sprint 3.

| Source and symbol | Observation for this fold |
|---|---|
| `deploy/test/checks.sh:324`, `psql_in_stack`, and line 434, `check_c5` | The helper uses local `psql` with `lgtm` defaults and no password. Item 2 preserves the SQL input and uses the mounted application file. |
| `deploy/test/checks.sh:590`, `check_c12`, and `deploy/test/README.md`, `LGTM_TEST_POSTGRES` | C12 requires both the disposable connection and executable mutation hook. The Server store plan, item 4, owns `src/server/internal/store/check-feed-lock.sh`. The proposed `deploy/test/mutate-feed-lock.sh` follows the existing driver's `mutation_hook` call. |
| `deploy/test/checks.sh:610`, `server_key_fingerprint` | The restart check filters on `msg=link_signing_key_loaded`, `source=openbao` and a nonempty string `fingerprint`. The sprint 2 integration table records the existing reader. |
| `deploy/stack-prepare.sh`, `prepare-db`, `reconcile-db` and proposed `reset-state` | The targets are new. `floorplan/plan.json` names the Compose and vault targets. The existing reusable patterns are the named volume in `design/openpencil/compose.yml` and fixture setup in `deploy/test/checks.sh`. |
| `deploy/test/vault-testlib.sh`, `fixture_key_action`, and `deploy/test/vault-fixtures.sh` | These targets are new. The retained fixture runner follows `deploy/test/checks.sh`, `run_check` and `cleanup`. |
| `deploy/openbao-provision.sh`, `admin/operator.json`, and `deploy/vault-admin.sh`, `admin/rotation.state` | These targets are new. The nearby upstream contracts are [AppRole](https://openbao.org/docs/auth/approle/) and [OpenBao policies](https://openbao.org/docs/concepts/policies/). The operator's direct token permissions and the authority it gets through policy changes are different. The protected file is administrator material. |
| `deploy/openbao-tools/Dockerfile`, `mv --no-copy -T` | GNU Coreutils implements rename without copy fallback. The release Dockerfile specifies an Alpine base. This confirms the source behavior. The package pin and production helper build remain required observations. [GNU Coreutils 9.7 source](https://github.com/coreutils/coreutils/blob/v9.7/src/mv.c), [OpenBao 2.7.3 Dockerfile](https://github.com/openbao/openbao/blob/v2.7.3/Dockerfile). |
| PostgreSQL entrypoint, `docker_setup_env` and `_main` | The upstream script reads the password file before the conditional root user switch. It accepts an existing nonroot user and supplies a password to initialization. Item 2 checks the exact pinned image under user 70. [PostgreSQL image entrypoint](https://github.com/docker-library/postgres/blob/master/docker-entrypoint.sh). |
| PostgreSQL 16, `pg_hba_file_rules` | The view describes the current file, including parse errors. It does not prove which rules the backend loaded. The plan requires the file check and separate wrong-password tests. [PostgreSQL 16 system view](https://www.postgresql.org/docs/16/view-pg-hba-file-rules.html). |
| Docker bridge gateway modes, `internal` | The documented ordinary internal bridge can reach host services on the bridge address. The threat reviewer reproduced this with a disposable listener. PB9 records the result on the target host. [Docker bridge behavior](https://docs.docker.com/engine/network/port-publishing/#gateway-modes). |

The threat reviewer ran the fresh and retained PostgreSQL rule cases in disposable containers. The initialization flags changed the fresh rules, and an older volume retained its original trust rule. The same report observed a password passed via a Docker command argument in the process list. No reviewer reran OpenBao in round 2. This session ran `docker info` and received permission denied at the local socket. The earlier OpenBao observations in this file remain reviewer evidence. The registry and production-shape replay remain build gates.

The implementation units stay in Stack's `deploy/` scope. The orchestrator should carry the fixed database setting names, the signing-key log event and the policy handoff into the later Server and Stack plans.

## Round 3 sources and current anchors

The two reports reviewed the round 2 plan. The architecture report also read four small changes after that snapshot. The findings refer to the larger plan at `attack/plan-v3.md` and its nearby working version. The earlier sections of scouting record historical evidence. Their removed targets and proof variations do not assign new build work. [Architecture review](attack/round-3-architect.md), [logic review](attack/round-3-logic.md).

| Source or target and anchor | Observation |
|---|---|
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md:215`, Secrets live in OpenBao, and lines 378 to 390 | The owner requires PostgreSQL storage, static automatic unseal, a restricted server credential and internal service access. The change-feed and restart checks require neither a maintenance role nor scripted data removal. |
| `design/openpencil/compose.yml`, `volumes.caddy-data`, and `floorplan/plan.json`, `deploy/compose.yaml`, `deploy/openbao.hcl` and `deploy/openbao-init.sh` | The existing Compose file anchors project volumes. The floorplan names the three planned stack files. `deploy/postgres-init.sql`, `deploy/stack-prepare.sh` with `prepare-db`, `reconcile-db` and `seal`, and `deploy/openbao-start.sh` are planned additions. Follow the PostgreSQL image's init pattern and the OpenBao configuration reviewed above. |
| `deploy/test/checks.sh:324`, `psql_in_stack`, and `deploy/test/README.md:63` | The existing query uses the two `lgtm` defaults. Stack item 2 owns the complete replacement and the test guide entry. The existing driver's `compose` and `run_check` are also the patterns for the planned `deploy/up.sh`, `deploy/test/postgres.sh`, `deploy/test/seal.sh`, `deploy/test/vault-init.sh` and `deploy/test/stack-vault.sh`. |
| `deploy/test/checks.sh:11`, `mutation_hook`, and line 590, `check_c12` | The driver requires the setting and executable. The planned scripts are `deploy/test/with-postgres.sh`, `deploy/test/mutate-feed-lock.sh` and their test `deploy/test/vault-fixtures.sh`. The Store plan's `NewDatabase` entry requires database `lgtm_store_test` and the database-creation privilege. Its feed item owns the script the hook invokes. |
| `deploy/test/checks.sh:604`, `vault_ready`, and line 619, `check_c13` | The check needs both auto-unseal and a server fingerprint. The local sprint 1 suite verifies the vault portion. The full driver recreates the running services and then checks the website. |
| `deploy/openbao-init.sh`, `admin/init-response.json`, `server/openbao.json` and `deploy/openbao-server-policy.hcl` | The pinned CLI exposes structured init and AppRole results. The protected response is the administrator material. The separate runtime file contains the restricted role and secret identifiers. The host already uses `jq` in the rollout driver. [OpenBao operator init](https://openbao.org/docs/commands/operator/init/), [OpenBao write](https://openbao.org/docs/commands/write/), [OpenBao AppRole](https://openbao.org/docs/auth/approle/). |
| `.plans/plans/go-s2-server-signin/plan.md`, server settings and signing key, and `.plans/plans/go-s2-stack-web/decisions.md`, rows 18, 24 and 25 | Sign-in now names all five PostgreSQL settings and the credential and event readers. The web plan still uses a combined connection setting and anticipates another C5 edit. The sprint 2 fallback covers the launcher flag. Relay these changes through the orchestrator. |

The architecture reviewer observed PostgreSQL starting as numeric user 70 with password authentication and no added capabilities. That reviewer also observed the OpenBao base image and temporary package behavior. The fold no longer relies on the helper package repository. The earlier reviewer runs establish the observed daemon behavior. This session received permission denied from the local Docker socket when running `docker info`. The pinned CLI path, clean launch and end-to-end restart remain build gates.
