#!/usr/bin/env bash
set -euo pipefail
umask 077

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
state=${LGTM_STACK_STATE_DIR:-"$here/.state"}
if [[ $state != /* ]]; then
    printf '%s\n' 'LGTM_STACK_STATE_DIR must be absolute.' >&2
    exit 1
fi
mkdir -p -m 0700 -- "$state"
LGTM_STACK_STATE_DIR=$(cd -- "$state" && pwd -P)
export LGTM_STACK_STATE_DIR

# Lock the directory inode so another spelling of the same path shares the lock.
exec {state_lock}<"$LGTM_STACK_STATE_DIR"
flock --exclusive "$state_lock"

compose() {
    docker compose -f "$here/compose.yaml" "$@"
}

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
phase ready true
printf '%s\n' 'postgres and openbao are healthy'
