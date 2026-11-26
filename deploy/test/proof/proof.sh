#!/usr/bin/env bash
set +x
set -euo pipefail
umask 077

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
pg_image=postgres:16.16-alpine@sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d
bao_image=openbao/openbao:2.7.3@sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9
scratch=$(mktemp -d)
project="lgtm-s1v-proof-${scratch##*.}"
label="lgtm.s1v.fixture=$project"
pg="$project-pg"
bao="$project-bao"
client="$project-client"
stage="prepare the fixture"

# Keep command diagnostics private. Only print observations that carry no credentials.
exec 3>&2 2>"$scratch/diagnostics"

cleanup() {
    local status=$? containers failed=0 resources remaining
    local -a ids list
    trap - EXIT
    trap '' INT TERM
    if containers=$(docker ps -aq --filter "label=$label"); then
        if [[ -n $containers ]]; then
            mapfile -t ids <<< "$containers"
            docker rm -f "${ids[@]}" >/dev/null || failed=1
        fi
    else
        failed=1
    fi
    if [[ -n ${volume_id:-} ]]; then
        docker volume rm "$volume_id" >/dev/null || failed=1
    fi
    if [[ -n ${network_id:-} ]]; then
        docker network rm "$network_id" >/dev/null || failed=1
    fi
    for resources in 'container ps -aq' 'volume ls -q' 'network ls -q'; do
        read -r -a list <<< "$resources"
        remaining=$(docker "${list[@]}" --filter "label=$label") || failed=1
        [[ -z $remaining ]] || failed=1
    done
    rm -rf -- "$scratch" || failed=1
    if (( status != 0 )); then
        printf 'proof failed while attempting to %s (status %s)\n' "$stage" "$status" >&3
    fi
    if (( failed != 0 )); then
        printf 'proof cleanup failed\n' >&3
        exit 1
    fi
    printf 'cleanup=true\n'
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

random_hex() {
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
    printf '\n'
}

inspect_image() {
    local service=$1 ref=$2 repository=$3 fallback_user=$4 details
    details=$(docker image inspect --format '{{.Os}}/{{.Architecture}} {{json .RepoDigests}} {{.Config.User}} {{json .Config.Entrypoint}} {{json .Config.Cmd}}' "$ref")
    local platform digests user entrypoint command
    read -r platform digests user entrypoint command <<< "${details/\]  /\] $fallback_user }"
    jq -e --arg digest "$repository@${ref##*@}" 'index($digest) != null' <<< "$digests" >/dev/null
    if [[ -n $fallback_user ]]; then
        printf '%s image default user is unset; fixture runs as %s\n' "$service" "$fallback_user"
    fi
    printf 'image %s %s %s@%s user=%s entrypoint=%s cmd=%s\n' \
        "$service" "$platform" "$repository" "${ref##*@}" "$user" "$entrypoint" "$command"
}

retry() {
    local deadline=$((SECONDS + 60))
    until "$@"; do
        (( SECONDS < deadline )) || return 1
        sleep 1
    done
    (( SECONDS < deadline ))
}

start_pg() {
    docker run -d --name "$pg" --label "$label" --network "$project" --network-alias proof-pg \
        --user 70:70 --cap-drop ALL --security-opt no-new-privileges:true \
        --env PGDATA=/var/lib/postgresql/data/pgdata \
        --env POSTGRES_INITDB_ARGS='--auth-host=scram-sha-256 --auth-local=scram-sha-256' \
        --env POSTGRES_HOST_AUTH_METHOD=scram-sha-256 \
        --env POSTGRES_PASSWORD_FILE=/run/lgtm-db/postgres.password \
        --mount "type=bind,source=$scratch/db,target=/run/lgtm-db,readonly" \
        --mount "type=volume,source=$volume_id,target=/var/lib/postgresql/data" \
        "$pg_image" >/dev/null
}

pg_sql() {
    local login=$1 database=$2
    shift 2
    timeout 5 docker exec -i "$pg" sh -ec '
        export PGPASSWORD="$(cat "/run/lgtm-db/$1.password")"
        export PROOF_BAO_PASSWORD="$(cat /run/lgtm-db/lgtm_bao.password)"
        exec psql -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -U "$@"
    ' sh "$login" -d "$database" "$@"
}

pg_ready() {
    [[ $(pg_sql postgres postgres -Atqc 'SELECT 1') == 1 ]]
}

start_bao() {
    docker run -d --name "$bao" --label "$label" --network "$project" --network-alias proof-bao \
        --user 10002:10002 --cap-drop ALL --security-opt no-new-privileges:true \
        --mount "type=bind,source=$scratch/keys,target=/run/lgtm-bao,readonly" \
        --mount "type=bind,source=$scratch/bao.password,target=/run/lgtm-db/postgres-bao.password,readonly" \
        --mount "type=bind,source=$scratch/bao.hcl,target=/openbao/config/lgtm.hcl,readonly" \
        --mount "type=bind,source=$scratch/openbao-start.sh,target=/openbao-start.sh,readonly" \
        --entrypoint /bin/sh "$bao_image" /openbao-start.sh server >/dev/null
}

bao_cli() {
    timeout 5 docker exec "$client" bao "$@"
}

with_token() {
    timeout 5 docker exec -i "$client" sh -ec '
        IFS= read -r BAO_TOKEN
        export BAO_TOKEN
        exec bao "$@"
    ' sh "$@" < "$scratch/token"
}

bao_status() {
    local status=0
    bao_cli status -format=json > "$scratch/status.json" || status=$?
    [[ $status == 0 || $status == 2 ]]
}

assert_unsealed() {
    bao_status && jq -e '.initialized == true and .sealed == false and .storage_type == "postgresql"' \
        "$scratch/status.json" >/dev/null
}

assert_wrong_key_sealed() {
    bao_status && jq -e '.initialized == true and .sealed == true and .storage_type == "postgresql"' \
        "$scratch/status.json" >/dev/null
}

row_count() {
    pg_sql lgtm_bao lgtm_bao -Atqc 'SELECT count(*) FROM public.openbao_kv_store'
}

table_absent() {
    [[ $(pg_sql lgtm_bao lgtm_bao -Atqc "SELECT to_regclass('public.openbao_kv_store') IS NULL") == t ]]
}

expected_sample=postgres-proof
assert_sample() {
    [[ $(with_token kv get -mount=proof -field=value sample) == "$expected_sample" ]]
}

preserved_rows() {
    [[ $(row_count) == "$after" ]]
}

stage="pull and inspect the pinned images"
docker pull "$pg_image" >/dev/null
docker pull "$bao_image" >/dev/null

stage="prepare PostgreSQL and the mounted credentials"
mkdir -m 0755 "$scratch/keys" "$scratch/db"
random_hex > "$scratch/db/postgres.password"
random_hex > "$scratch/db/lgtm_bao.password"
cp "$scratch/db/lgtm_bao.password" "$scratch/bao.password"
head -c 32 /dev/urandom > "$scratch/keys/unseal.key"
head -c 32 /dev/urandom > "$scratch/keys/wrong.key"
if cmp -s "$scratch/keys/unseal.key" "$scratch/keys/wrong.key"; then
    exit 1
fi
cp "$here/bao.hcl" "$scratch/bao.hcl"
cat > "$scratch/openbao-start.sh" <<'SH'
set -eu
IFS= read -r password < /run/lgtm-db/postgres-bao.password
export BAO_PG_CONNECTION_URL="postgresql://lgtm_bao:$password@proof-pg:5432/lgtm_bao?sslmode=disable"
exec bao "$@" -config=/openbao/config/lgtm.hcl
SH
chmod 0444 "$scratch/bao.hcl" "$scratch/openbao-start.sh"
network_id=$(docker network create --internal --label "$label" "$project")
volume_id=$(docker volume create --label "$label" "$pg")
docker run --rm --name "$project-prepare" --label "$label" --network none \
    --mount "type=bind,source=$scratch,target=/fixture" \
    --mount "type=volume,source=$volume_id,target=/var/lib/postgresql/data" \
    --entrypoint /bin/sh "$pg_image" -ec '
        chown 70:70 /var/lib/postgresql/data /fixture/db/postgres.password /fixture/db/lgtm_bao.password
        chown 10002:10002 /fixture/keys/unseal.key /fixture/keys/wrong.key /fixture/bao.password
        chmod 0400 /fixture/keys/unseal.key /fixture/keys/wrong.key /fixture/bao.password \
            /fixture/db/postgres.password /fixture/db/lgtm_bao.password
    '
start_pg
retry pg_ready
[[ $(docker exec "$pg" awk '/^Uid:/ {uid=$3} /^Gid:/ {gid=$3} END {print uid ":" gid}' /proc/1/status) == 70:70 ]]
inspect_image postgres "$pg_image" postgres 70:70
inspect_image openbao "$bao_image" openbao/openbao ''
printf 'postgres effective_user=70:70\n'

stage="verify the lgtm_bao database and storage table before startup"
pg_sql postgres postgres >/dev/null <<'SQL'
\getenv bao_password PROOF_BAO_PASSWORD
CREATE ROLE lgtm_bao LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'bao_password';
CREATE DATABASE lgtm_bao OWNER lgtm_bao;
REVOKE CONNECT ON DATABASE lgtm_bao, postgres, template1 FROM PUBLIC;
SQL
[[ $(pg_sql lgtm_bao lgtm_bao -Atqc "
    SELECT pg_get_userbyid(d.datdba) = current_user
        AND r.rolcanlogin AND NOT (r.rolsuper OR r.rolcreatedb OR r.rolcreaterole)
    FROM pg_database d JOIN pg_roles r ON r.rolname = current_user
    WHERE d.datname = current_database()
") == t ]]
table_absent

stage="initialize OpenBao over the internal network"
docker run -d --name "$client" --label "$label" --network "$project" \
    --user 10002:10002 --cap-drop ALL --security-opt no-new-privileges:true \
    --env BAO_ADDR=http://proof-bao:8200 --entrypoint sleep "$bao_image" infinity >/dev/null
start_bao
retry bao_status
[[ $(pg_sql lgtm_bao lgtm_bao -Atqc "
    SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid = 'public.openbao_kv_store'::regclass
") == lgtm_bao ]]
bao_cli operator init -format=json -recovery-shares=1 -recovery-threshold=1 > "$scratch/init.json"
jq -er '.root_token | strings | select(length > 0)' "$scratch/init.json" > "$scratch/token"
retry assert_unsealed

stage="write a sample and observe PostgreSQL row growth"
with_token secrets enable -path=proof kv-v2 >/dev/null
before=$(row_count)
with_token kv put -mount=proof sample "value=$expected_sample" >/dev/null
after=$(row_count)
[[ $before =~ ^[0-9]+$ && $after =~ ^[0-9]+$ && $after -gt $before ]]
printf 'rows before=%s after=%s\n' "$before" "$after"

stage="replace PostgreSQL and OpenBao and verify automatic unseal"
docker rm -f "$bao" "$pg" >/dev/null
start_pg
retry pg_ready
start_bao
retry assert_unsealed
printf 'restart initialized=true sealed=false storage_type=postgresql\n'
printf 'assertion static_seal_restart pass\n'
stage="read the sample after replacing both containers"
assert_sample
printf 'assertion postgres_storage_round_trip pass\n'

stage="verify the missing key prevents startup and preserves the rows"
docker rm -f "$bao" >/dev/null
mv -- "$scratch/keys/unseal.key" "$scratch/original.key"
start_bao
[[ $(timeout 60 docker wait "$bao") == 1 ]]
docker logs "$bao" > "$scratch/missing.log" 2>&1
grep -Eq 'unseal\.key: no such file or directory' "$scratch/missing.log"
if assert_unsealed; then
    exit 1
fi
preserved_rows

stage="verify a different 32-byte key keeps the initialized vault sealed"
docker rm -f "$bao" >/dev/null
mv -- "$scratch/keys/wrong.key" "$scratch/keys/unseal.key"
start_bao
retry assert_wrong_key_sealed
for ((until=SECONDS + 5; SECONDS < until;)); do
    sleep 1
    assert_wrong_key_sealed
done
preserved_rows

stage="restore the original key and verify recovery of the sample"
docker rm -f "$bao" >/dev/null
mv -- "$scratch/original.key" "$scratch/keys/unseal.key"
start_bao
retry assert_unsealed
assert_sample
printf 'missing key: startup exited, the key file was absent, and rows were preserved\n'
printf 'wrong key: initialized=true sealed=true storage_type=postgresql, and rows were preserved\n'
printf 'restoring the original key recovered the sample\n'
printf 'assertion missing_seal_key_blocks_unseal pass\n'
printf 'assertion wrong_seal_key_stays_sealed pass\n'
