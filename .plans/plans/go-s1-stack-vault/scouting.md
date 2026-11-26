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

## Sources for files the build creates

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
