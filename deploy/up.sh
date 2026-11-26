#!/usr/bin/env bash
# Start the stack. With no argument, the launcher starts PostgreSQL, the vault, the server and the
# web server. With --until vault, it stops once PostgreSQL and the vault are healthy.
set +x
set -euo pipefail
umask 077

usage() {
    printf '%s\n' 'usage: deploy/up.sh [--until vault]' >&2
    exit 2
}

until_vault=0
case $# in
    0) ;;
    2) [[ $1 == --until && $2 == vault ]] || usage; until_vault=1 ;;
    *) usage ;;
esac

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
state=${LGTM_STACK_STATE_DIR:-"$here/.state"}
if [[ $state != /* ]]; then
    printf '%s\n' 'LGTM_STACK_STATE_DIR must be absolute.' >&2
    exit 1
fi

# The vault route needs no valid web port, so every Compose call of that route reads a fixed one.
if ((until_vault)); then
    export LGTM_WEB_PORT=8787
fi

# With -f, Compose reads deploy/.env and takes the project name from it, whatever folder the
# launcher runs from. A list of files in COMPOSE_FILE replaces -f, so a test's overrides reach
# every call.
compose() {
    if [[ -n ${COMPOSE_FILE:-} ]]; then
        docker compose "$@"
    else
        docker compose -f "$here/compose.yaml" "$@"
    fi
}

# The settings can change while the launcher waits for the lock. Every Compose call, including the
# calls of stack-prepare.sh, reads one private copy, which is removed when the launcher exits.
settings_copy() {
    local -a files
    local file n=0 list=''
    if [[ -n ${COMPOSE_ENV_FILES:-} ]]; then
        IFS=, read -r -a files <<<"$COMPOSE_ENV_FILES"
    else
        case ${COMPOSE_DISABLE_ENV_FILE:-} in
            1|t|T|true|TRUE|True) files=() ;;
            *) file=${COMPOSE_FILE:-"$here/compose.yaml"}
               file=${file%%"${COMPOSE_PATH_SEPARATOR:-:}"*}
               files=("$(dirname -- "$file")/.env")
               [[ -e ${files[0]} ]] || files=() ;;
        esac
    fi
    : >"$settings/0" || return 1
    list=$settings/0
    for file in "${files[@]}"; do
        n=$((n + 1))
        cat -- "$file" >"$settings/$n" 2>/dev/null || return 1
        list=$list,$settings/$n
    done
    export COMPOSE_ENV_FILES=$list
}

settings=$(mktemp -d)
trap 'rm -rf -- "$settings"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# Compose's own message for a malformed settings file can print the password, so it is discarded.
if ! settings_copy || ! compose config --quiet >/dev/null 2>&1; then
    printf '%s\n' 'the settings cannot be read; check the quoting in deploy/.env' >&2
    exit 2
fi

mkdir -p -m 0700 -- "$state"
LGTM_STACK_STATE_DIR=$(cd -- "$state" && pwd -P)
export LGTM_STACK_STATE_DIR

# Lock the directory inode so another spelling of the same path shares the lock.
exec {state_lock}<"$LGTM_STACK_STATE_DIR"
flock --exclusive "$state_lock"

wait_healthy() {
    local service container health deadline=$((SECONDS + 60))
    for service in "$@"; do
        container=$(compose ps --all --quiet "$service")
        if [[ -z $container ]]; then
            printf 'The %s container is missing.\n' "$service" >&2
            return 1
        fi
        while true; do
            health=$(docker inspect --format '{{.State.Health.Status}}' "$container")
            [[ $health != healthy ]] || break
            if (( SECONDS >= deadline )); then
                printf 'The %s container did not become healthy.\n' "$service" >&2
                return 1
            fi
            sleep 1
        done
    done
}

prepare_database() {
    "$here/stack-prepare.sh" prepare-db
    compose up -d postgres
    "$here/stack-prepare.sh" reconcile-db
    wait_healthy postgres
}

start_vault() {
    compose up -d openbao
    compose --profile setup run --rm --no-deps -T openbao-init
    wait_healthy postgres openbao
}

build_images() {
    compose build server web
}

# A first start may migrate the database for the server's full budget. A server that exits
# ends the wait at once; its last lines are shown, and it is stopped so that it does not restart.
start_server() {
    if ! compose up -d --wait --wait-timeout 540 --no-deps server; then
        compose logs --no-log-prefix --tail 20 server || true
        compose stop server || true
        exit 1
    fi
}

# Compose's output names a taken port. The failed web container is removed, so that the next run
# creates it again with its port, and the other services keep running.
start_web() {
    if ! compose up -d --wait --wait-timeout 90 --no-deps web; then
        compose rm --force --stop web >/dev/null 2>&1 || true
        exit 1
    fi
}

find_address() {
    address=$(compose port web 8080)
    [[ -n $address ]]
}

phase() {
    local name=$1
    shift
    printf 'phase %s start\n' "$name"
    "$@"
    printf 'phase %s done\n' "$name"
}

phase database prepare_database
phase seal "$here/stack-prepare.sh" seal
phase vault start_vault
if ((until_vault)); then
    phase ready true
    printf '%s\n' 'postgres and openbao are healthy'
    exit 0
fi
phase images build_images
phase server start_server
phase web start_web
phase ready find_address
printf '%s\n' 'postgres and openbao are healthy'
printf 'the website is at http://%s/\n' "$address"
