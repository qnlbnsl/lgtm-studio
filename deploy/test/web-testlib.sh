#!/usr/bin/env bash
# Source these helpers to run a test against its own Compose project and state.

web_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P)
web_caddy=caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0

web_fixture_random() {
    od -An -N"$1" -tx1 /dev/urandom | tr -d ' \n'
}

web_fixture_create() {
    web_tmp=$(cd -- "${TMPDIR:-/tmp}" && pwd -P) || return 1
    web_folder=$(mktemp -d "${web_tmp%/}/tmp.XXXXXXXXXX") || return 1
    local suffix
    suffix=$(web_fixture_random 4) || return 1
    [[ $suffix =~ ^[0-9a-f]{8}$ ]] || return 1
    web_project=lgtm-s2w-$suffix
    export COMPOSE_PROJECT_NAME=$web_project LGTM_IMAGE_TAG=$web_project
    export LGTM_STACK_STATE_DIR=$web_folder/state COMPOSE_ENV_FILES=$web_folder/settings.env
    export COMPOSE_FILE=$web_root/deploy/compose.yaml COMPOSE_PATH_SEPARATOR=:
    unset LGTM_WEB_PASSWORD LGTM_WEB_PORT COMPOSE_PROFILES COMPOSE_DISABLE_ENV_FILE

    local attempt port
    for ((attempt = 0; attempt < 40; attempt++)); do
        port=$((20000 + RANDOM % 20000))
        if ! (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
            web_port=$port
            break
        fi
    done
    [[ $attempt -lt 40 ]] || return 1
    web_password=$(web_fixture_random 16) || return 1
    [[ $web_password =~ ^[0-9a-f]{32}$ ]] || return 1
    (umask 077; printf 'LGTM_WEB_PORT=%s\nLGTM_WEB_PASSWORD=%s\n' "$web_port" "$web_password" >"$COMPOSE_ENV_FILES")
}

web_fixture_override() {
    local file
    file=$(realpath -- "$1") || return 1
    [[ -f $file && $file != *:* ]] || return 1
    export COMPOSE_FILE=$COMPOSE_FILE:$file
}

web_fixture_remove_project() {
    local project=$1 object
    [[ $project =~ ^lgtm-s2w-[0-9a-f]{8}$ ]] || return 1
    # The live test model may have added an override or an isolated copy of the deployment.
    for object in $(docker ps -aq --filter "label=com.docker.compose.project=$project"); do
        docker rm -fv "$object" >/dev/null || return 1
    done
    for object in $(docker network ls -q --filter "label=com.docker.compose.project=$project"); do
        docker network rm "$object" >/dev/null || return 1
    done
    for object in $(docker volume ls -q --filter "label=com.docker.compose.project=$project"); do
        docker volume rm "$object" >/dev/null || return 1
    done
}

web_fixture_cleanup() {
    [[ ${web_folder:-} == "${web_tmp%/}"/tmp.* &&
        ${web_folder%/*} == "${web_tmp%/}" && -d $web_folder && ! -L $web_folder ]] || return 1
    if [[ ${web_project:-} =~ ^lgtm-s2w-[0-9a-f]{8}$ ]]; then
        # Use a fresh settings file even if the test deliberately broke its own.
        (umask 077; : >"$web_folder/cleanup.env")
        COMPOSE_ENV_FILES=$web_folder/cleanup.env LGTM_WEB_PORT=0 \
            docker compose -p "$web_project" -f "$web_root/deploy/compose.yaml" \
            down -v --remove-orphans -t 5 >/dev/null 2>&1 || true
        web_fixture_remove_project "$web_project" || return 1
        local object
        for object in "lgtm-server:$web_project" "lgtm-web:$web_project"; do
            if docker image inspect "$object" >/dev/null 2>&1; then
                docker image rm "$object" >/dev/null || return 1
            fi
        done
        if [[ -d $web_folder/state ]]; then
            docker run --rm --name "$web_project-cleanup" --user 0:0 --network none \
                --volume "$web_folder:/fixture" --entrypoint sh "$web_caddy" \
                -ec 'find /fixture -mindepth 1 -depth -delete' || return 1
        fi
    fi
    rm -rf -- "$web_folder"
}

web_fixture_stat() {
    local path=${1#"$LGTM_STACK_STATE_DIR"/}
    [[ $path != /* && /$path/ != */../* ]] || return 1
    docker run --rm --name "$web_project-stat" --user 0:0 --network none \
        --volume "$web_folder:/fixture:ro" --entrypoint stat "$web_caddy" \
        -c '%u:%g %a' "/fixture/state/$path"
}

# Check the fixture owns every named Compose resource before starting containers.
web_fixture_owns_model() {
    jq -e --arg project "$web_project" '
        def owned: type == "string" and (startswith($project + "-") or startswith($project + "_"));
        .name == $project
        and ([.networks // {}, .volumes // {}] | all(.[]; all(.[]; (.external != true) and (.name | owned))))
        and (.services | all(.[]; (.container_name == null or (.container_name | owned))))
    ' >/dev/null
}

web_check() {
    local name=$1
    if ("$name"); then
        printf 'PASS [%s]\n' "$name"
        ((web_passed += 1))
    else
        printf 'FAIL [%s]\n' "$name"
        ((web_failed += 1))
    fi
}

web_finish() {
    printf '%d passed, %d failed\n' "$web_passed" "$web_failed"
    ((web_failed == 0))
}
