#!/usr/bin/env bash
# Shared helpers of the tests of the sprint 1 stack (postgres.sh, seal.sh,
# vault-init.sh and stack-vault.sh). Source this file; do not run it.
#
# Every Docker object of a test run belongs to a Compose project whose name
# starts with lgtm-s1v-. Every project has its own state folder, made by
# mktemp -d. Nothing here touches deploy/.state. Cleanup runs on exit.

set -u
set -o pipefail
export LC_ALL=C

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
PGIMG='postgres:16.16-alpine@sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d'
BAOIMG='openbao/openbao:2.7.3@sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9'

declare -A NCHECKS=() FAILED=()
ASSERTIONS=()
PASSED=0
FAILCOUNT=0
STACKS=()
P=''
STATE=''
OUT=''
RC=0
WORK=$(mktemp -d)
umask 077

# check <assertion> <description> <command...> records one expectation.
check() {
    local assertion=$1 description=$2
    shift 2
    NCHECKS[$assertion]=$(( ${NCHECKS[$assertion]:-0} + 1 ))
    if "$@" >/dev/null 2>&1; then
        PASSED=$((PASSED + 1))
        printf 'PASS [%s] %s\n' "$assertion" "$description"
        return 0
    else
        FAILED[$assertion]=1
        FAILCOUNT=$((FAILCOUNT + 1))
        printf 'FAIL [%s] %s\n' "$assertion" "$description"
        return 1
    fi
}

# finish prints one line per named assertion and exits.
finish() {
    local assertion bad=0
    for assertion in "${ASSERTIONS[@]}"; do
        if [[ -z ${FAILED[$assertion]:-} && ${NCHECKS[$assertion]:-0} -gt 0 ]]; then
            printf 'assertion %s pass\n' "$assertion"
        else
            printf 'assertion %s fail\n' "$assertion"
            bad=1
        fi
    done
    printf '%s passed, %s failed\n' "$PASSED" "$FAILCOUNT"
    exit "$bad"
}

rnd() { od -An -N4 -tx1 /dev/urandom | tr -d ' \n'; }

# ---------------------------------------------------------------------------
# Projects and state folders
# ---------------------------------------------------------------------------

stack_new() {
    P=lgtm-s1v-$(rnd)
    STATE=$(mktemp -d)
    STACKS+=("$P|$STATE")
}

stack_use() { P=$1; STATE=$2; }

envrun() { env LGTM_STACK_STATE_DIR="$STATE" COMPOSE_PROJECT_NAME="$P" "$@"; }

# dc runs Docker Compose on the stack's own file in the current project.
dc() { ( cd "$root" && envrun docker compose -f deploy/compose.yaml -p "$P" "$@" ); }

# sp runs the preparer on the host. OUT holds both output streams and RC the status.
sp() {
    OUT=$( cd "$root" && envrun timeout 300 deploy/stack-prepare.sh "$@" 2>&1 )
    RC=$?
}
sp_ok() { sp "$@"; [[ $RC -eq 0 ]]; }

# launch runs the launcher of item 6 the same way.
launch() {
    OUT=$( cd "$root" && envrun timeout 600 deploy/up.sh "$@" 2>&1 )
    RC=$?
}

stack_cleanup() {
    local entry p s id
    for entry in "${STACKS[@]}"; do
        p=${entry%%|*}
        s=${entry#*|}
        ( cd "$root" && env LGTM_STACK_STATE_DIR="$s" COMPOSE_PROJECT_NAME="$p" \
            docker compose -f deploy/compose.yaml -p "$p" --profile '*' down -v --remove-orphans ) >/dev/null 2>&1
        for id in $(docker ps -aq --filter "label=com.docker.compose.project=$p" 2>/dev/null); do
            docker rm -f -v "$id" >/dev/null 2>&1
        done
        for id in $(docker network ls -q --filter "label=com.docker.compose.project=$p" 2>/dev/null); do
            docker network rm "$id" >/dev/null 2>&1
        done
        for id in $(docker volume ls -q --filter "label=com.docker.compose.project=$p" 2>/dev/null); do
            docker volume rm -f "$id" >/dev/null 2>&1
        done
        docker volume rm -f "${p}_pgdata" >/dev/null 2>&1
        docker run --rm --name "lgtm-s1v-x-$(rnd)" --user 0:0 --network none -v "$s:/s" \
            --entrypoint sh "$PGIMG" -c 'find /s -mindepth 1 -delete' >/dev/null 2>&1
        rmdir -- "$s" 2>/dev/null
    done
    rm -rf -- "$WORK"
}
trap stack_cleanup EXIT

# ---------------------------------------------------------------------------
# Waiting
# ---------------------------------------------------------------------------

# await <seconds> <command...> polls until the command succeeds.
await() {
    local limit=$1
    shift
    local deadline=$((SECONDS + limit))
    until "$@" >/dev/null 2>&1; do
        ((SECONDS < deadline)) || return 1
        sleep 2
    done
}

cid_of() { dc ps -a -q "$1" 2>/dev/null | head -n 1; }

health_of() {
    local c
    c=$(cid_of "$1")
    [[ -n $c ]] || { echo missing; return 0; }
    docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$c" 2>/dev/null
}
is_healthy() { [[ $(health_of "$1") == healthy ]]; }
never_healthy() { # <service> <seconds>
    local deadline=$((SECONDS + $2))
    while ((SECONDS < deadline)); do
        is_healthy "$1" && return 1
        sleep 2
    done
    return 0
}

# ---------------------------------------------------------------------------
# The state folder, read and written as its administrator
# ---------------------------------------------------------------------------

# rootsh <script> [arguments] runs a script as root with the state folder at /s.
rootsh() {
    local script=$1
    shift
    docker run --rm -i --name "lgtm-s1v-h-$(rnd)" --user 0:0 --network none -v "$STATE:/s" \
        --entrypoint sh "$PGIMG" -c "$script" _ "$@"
}
sstat() { rootsh 'stat -c "%u:%g %a" "/s/$1"' "$1"; }
sexists() { rootsh 'test -e "/s/$1"' "$1"; }
scat() { rootsh 'cat "/s/$1"' "$1"; }
ssize() { rootsh 'wc -c < "/s/$1"' "$1" | tr -d ' \n'; }
# sfp prints a fingerprint of a file: its digest and its modification time.
# sfp1 prints only the digest line of sfp. Reading the whole output avoids a broken pipe in the container client.
sfp1() { local o; o=$(sfp "$1") || return 1; printf '%s\n' "${o%%$'\n'*}"; }
sfp() { rootsh 'sha256sum "/s/$1" | cut -d" " -f1; stat -c %y "/s/$1"' "$1"; }
# sput writes standard input into a file in place. A new file gets owner and mode from the arguments.
sput() { rootsh 'cat > "/s/$1"' "$1"; }
sown() { rootsh 'chown "$2" "/s/$1" && chmod "$3" "/s/$1"' "$1" "$2" "$3"; }
srm() { rootsh 'rm -f "/s/$1"' "$1"; }
smv() { rootsh 'mv "/s/$1" "/s/$2"' "$1" "$2"; }
# sfind prints the path, relative to the state folder, of the file of the given name.
sfind() { rootsh 'find /s -name "$1" -type f | head -n 1 | sed "s|^/s/||"' "$1"; }

hex64() { head -c32 /dev/urandom | od -An -tx1 | tr -d ' \n'; }

# ---------------------------------------------------------------------------
# PostgreSQL
# ---------------------------------------------------------------------------

pw_name() {
    case $1 in
        postgres) echo postgres-admin.password ;;
        lgtm_app) echo postgres-app.password ;;
        lgtm_bao) echo postgres-bao.password ;;
    esac
}

# pgq <role> <database> <sql> runs a query from a client on the data network.
pgq() {
    local f
    f=$(sfind "$(pw_name "$1")") && [[ -n $f ]] || return 1
    docker run --rm --name "lgtm-s1v-q-$(rnd)" --user 0:0 --network "${P}_data" -v "$STATE:/s:ro" \
        --entrypoint sh "$PGIMG" -c \
        'PGPASSWORD=$(cat "/s/$1") PGCONNECT_TIMEOUT=10 exec psql -X -qtA -h postgres -U "$2" -d "$3" -v ON_ERROR_STOP=1 -c "$4"' \
        _ "$f" "$1" "$2" "$3"
}

# pgq_with <role> <database> <sql> <password> uses the given password instead of the stored one.
pgq_with() {
    docker run --rm --name "lgtm-s1v-q-$(rnd)" --user 0:0 --network "${P}_data" \
        --entrypoint sh "$PGIMG" -c \
        'PGPASSWORD=$4 PGCONNECT_TIMEOUT=10 exec psql -X -qtA -h postgres -U "$1" -d "$2" -v ON_ERROR_STOP=1 -c "$3"' \
        _ "$1" "$2" "$3" "$4"
}

# up_db runs the whole database part of the launch by hand: item 2 only.
up_db() {
    sp_ok prepare-db &&
        dc up -d postgres >/dev/null 2>&1 &&
        sp_ok reconcile-db &&
        await 120 is_healthy postgres
}

kv_rows() {
    local there
    there=$(pgq lgtm_bao lgtm_bao "select to_regclass('public.openbao_kv_store') is not null" | tr -d ' \n')
    if [[ $there == t ]]; then
        pgq lgtm_bao lgtm_bao 'select count(*) from public.openbao_kv_store' | tr -d ' \n'
    else
        echo 0
    fi
}

# ---------------------------------------------------------------------------
# OpenBao
# ---------------------------------------------------------------------------

# bao_run <token> <arguments> runs the client of the pinned image on the vault network.
bao_run() {
    local tok=$1
    shift
    BAO_TOKEN=$tok docker run --rm --name "lgtm-s1v-b-$(rnd)" --network "${P}_vault" \
        -e BAO_ADDR=http://openbao:8200 ${tok:+-e BAO_TOKEN} --entrypoint bao "$BAOIMG" "$@"
}

# bao_in <token> <arguments> runs the client inside the daemon's own container.
bao_in() {
    local tok=$1
    shift
    BAO_TOKEN=$tok dc exec -T ${tok:+-e BAO_TOKEN} openbao bao "$@"
}

# bao status exits 2 for a sealed vault but still prints its JSON, so the status is dropped here.
bao_status() { dc exec -T openbao bao status -format=json 2>/dev/null || true; }
vault_open() { bao_status | jq -e '.initialized == true and .sealed == false' >/dev/null 2>&1; }
vault_sealed() { bao_status | jq -e '.initialized == true and .sealed == true' >/dev/null 2>&1; }
vault_uninitialized() { bao_status | jq -e '.initialized == false' >/dev/null 2>&1; }

# init_vault initializes the vault by hand, keeping the raw response in $WORK/init.json and the token in ROOT.
init_vault() {
    ROOT=''
    await 90 vault_uninitialized || return 1
    bao_in '' operator init -format=json -recovery-shares=1 -recovery-threshold=1 >"$WORK/init.json" 2>/dev/null || return 1
    ROOT=$(jq -r .root_token "$WORK/init.json") && [[ -n $ROOT ]] || return 1
    await 90 vault_open
}

# put_sample stores one value at lgtm/sample/probe, as the administrator.
put_sample() {
    bao_in "$ROOT" secrets enable -path=lgtm -version=2 kv >/dev/null 2>&1 || return 1
    bao_in "$ROOT" kv put -mount=lgtm sample/probe "v=$1" >/dev/null 2>&1
}
get_sample() { bao_in "$ROOT" kv get -mount=lgtm -field=v sample/probe 2>/dev/null; }

# root_token reads the administrator token from the saved response.
root_token() { scat admin/init-response.json | jq -r .root_token; }

# accessors prints how many secret identifiers the server role has.
accessors() {
    local tok out
    tok=$(root_token) || return 1
    out=$(bao_run "$tok" list -format=json auth/approle/role/lgtm-server/secret-id 2>/dev/null)
    if jq -e 'type == "array"' <<<"$out" >/dev/null 2>&1; then
        jq 'length' <<<"$out"
    else
        echo 0
    fi
}

# as_server <script> runs a script as user 10001 with the published credential. The script sees
# the login response in $login and runs with the token in BAO_TOKEN. The seed, if any, is in $SEED.
as_server() {
    SCRIPT=$1 SEED=${SEED:-} docker run --rm --name "lgtm-s1v-s-$(rnd)" --user 10001:10001 \
        --network "${P}_vault" -e BAO_ADDR=http://openbao:8200 -e SCRIPT -e SEED \
        -v "$STATE/server/openbao.json:/c:ro" --entrypoint sh "$BAOIMG" -c '
            c=$(tr -d "\n" </c)
            role=$(printf %s "$c" | sed -n "s/.*\"role_id\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p")
            secret=$(printf %s "$c" | sed -n "s/.*\"secret_id\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p")
            login=$(bao write -format=json auth/approle/login role_id="$role" secret_id="$secret") || { echo "login failed" >&2; exit 90; }
            BAO_TOKEN=$(printf %s "$login" | tr -d "\n" | sed -n "s/.*\"client_token\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p")
            export BAO_TOKEN
            eval "$SCRIPT"'
}

# Signing key scripts for as_server.
KEY_WRITE='printf "{\"options\":{\"cas\":0},\"data\":{\"private_seed_b64\":\"%s\"}}" "$SEED" | bao write -format=json lgtm/data/server/link/signing-key -'
KEY_READ='bao read -format=json lgtm/data/server/link/signing-key | tr -d "\n" | sed -n "s/.*\"private_seed_b64\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p"'

seed() { head -c32 /dev/urandom | base64 -w0; }

# ---------------------------------------------------------------------------
# Docker facts
# ---------------------------------------------------------------------------

inspect_json() { docker inspect "$(cid_of "$1")" 2>/dev/null; }

# proc_status <service> <field> prints a field of /proc/1/status in the container.
proc_status() { docker exec "$(cid_of "$1")" sh -c "sed -n 's/^$2:[[:space:]]*//p' /proc/1/status" 2>/dev/null; }

compose_config() { dc --profile '*' config --format json 2>/dev/null; }

neg() { ! "$@"; }

export root PGIMG BAOIMG WORK P STATE
export_fns() { export -f $(declare -F | awk '{print $3}'); }
export_fns
