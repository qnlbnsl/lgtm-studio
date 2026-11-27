#!/usr/bin/env bash
# Source this library in the shell that owns one private Sprint 4 stack.
# Run the launcher and proof drivers through sprint4_fixture_run.

set +x
s4_library_dir=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P) || return 1
s4_library=$s4_library_dir/${BASH_SOURCE[0]##*/}

s4_fail() {
    printf '%s\n' "$1" >&2
    return 1
}

s4_git() (
    local names
    names=$(git rev-parse --local-env-vars) || return 1
    # Git prints one environment variable name per line.
    unset $names
    export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
    git -c core.hooksPath=/dev/null "$@"
)

s4_source_clean() {
    local head changes
    head=$(s4_git -C "$s4_candidate" rev-parse --verify HEAD) &&
        changes=$(s4_git -C "$s4_candidate" status --porcelain=v1 --untracked-files=all) &&
        [[ $head == "$s4_commit" && -z $changes ]]
}

s4_checkouts() {
    local source=$1 object repo
    s4_git clone --quiet --shared --no-checkout -- "$source" "$s4_candidate" &&
        s4_git -C "$s4_candidate" switch --quiet --detach "$s4_commit" || return 1
    for object in fcfa96c3 89506b7c f6cbea30; do
        s4_git -C "$s4_candidate" update-ref "refs/sprint4/$object" "$object" || return 1
    done
    # Keep a private object store even when the caller uses Git alternates.
    s4_git -C "$s4_candidate" repack -a -d &&
        rm -- "$s4_candidate/.git/objects/info/alternates" &&
        s4_git -C "$s4_candidate" remote remove origin &&
        mkdir -- "$s4_legacy" &&
        (set -o pipefail; s4_git -C "$s4_candidate" archive f6cbea30 | tar -x -C "$s4_legacy") ||
        return 1

    repo=$s4_dir/origin
    s4_git init --quiet --initial-branch=main "$repo" &&
        printf '%s\n' 'Sprint 4 repository fixture.' >"$repo/README.md" &&
        s4_git -C "$repo" add README.md &&
        s4_git -C "$repo" -c user.name=Fixture -c user.email=fixture@example.invalid commit --quiet -m 'Create the base fixture.' &&
        s4_git clone --quiet --no-hardlinks -- "$repo" "$S4_REPO_A" &&
        s4_git -C "$S4_REPO_A" remote remove origin &&
        s4_git clone --quiet --no-hardlinks -- "$repo" "$S4_REPO_B" &&
        s4_git -C "$S4_REPO_B" switch --quiet -c topic &&
        s4_git -C "$S4_REPO_B" -c user.name=Fixture -c user.email=fixture@example.invalid             commit --quiet --allow-empty -m 'Advance the topic branch.' &&
        printf '%s\n' 'An uncommitted checkout change.' >>"$S4_REPO_B/README.md" &&
        s4_source_clean
}

s4_socket() {
    python3 - "$1" <<'PY'
import socket
import sys

with socket.socket() as probe:
    probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    try:
        probe.bind(("127.0.0.1", int(sys.argv[1])))
    except OSError:
        sys.exit(1)
    print(probe.getsockname()[1])
PY
}

s4_path_identity() {
    python3 - "$@" <<'PY'
import os
import sys

for path in sys.argv[1:]:
    if not os.path.isdir(path) or os.path.realpath(path) != path:
        sys.exit(1)
    st = os.stat(path)
    print(st.st_dev, st.st_ino)
PY
}

s4_daemon() {
    local context endpoint=${DOCKER_HOST:-} id
    if [[ -n ${DOCKER_CONTEXT:-} || -z $endpoint ]]; then
        context=$("$s4_docker" context show 2>/dev/null) &&
            endpoint=$("$s4_docker" context inspect --format '{{(index .Endpoints "docker").Host}}' \
                "$context" 2>/dev/null) || return 1
    fi
    [[ $endpoint == unix:///* ]] &&
        id=$("$s4_docker" info --format '{{.ID}}' 2>/dev/null) && [[ -n $id ]] || return 1
    printf '%s\n%s\n' "$id" "$endpoint"
}

s4_same_paths() {
    local actual
    actual=$(s4_path_identity "$s4_dir" "$s4_state") && [[ $actual == "$s4_paths" ]]
}

s4_overlay() {
    local service network
    printf 'services:\n'
    for service in postgres openbao openbao-init prepare server web; do
        printf '  %s:\n    labels:\n      lgtm.sprint4.run: "%s"\n' "$service" "$s4_run"
        case $service in
            server|web) printf '    build:\n      labels:\n        lgtm.sprint4.run: "%s"\n' "$s4_run" ;;
        esac
    done
    printf 'networks:\n'
    for network in app data edge vault; do
        printf '  %s:\n    labels:\n      lgtm.sprint4.run: "%s"\n' "$network" "$s4_run"
    done
    printf 'volumes:\n  pgdata:\n    labels:\n      lgtm.sprint4.run: "%s"\n' "$s4_run"
}

s4_model() {
    "$s4_docker" compose --profile '*' config --format json 2>/dev/null
}

s4_valid_model() {
    local tree=$1 tag=$2
    jq -e --arg tree "$tree" --arg state "$s4_state" --arg project "$s4_project"         --arg port "$s4_port" --arg tag "$tag" --arg run "$s4_run" '
        def labeled: .labels["lgtm.sprint4.run"] == $run;
        .name == $project
        and (.services | keys == ["openbao","openbao-init","postgres","prepare","server","web"])
        and (.services | all(.[]; labeled and .container_name == null))
        and (.volumes | keys == ["pgdata"])
        and (.volumes.pgdata | labeled and .name == ($project + "_pgdata")
            and .external != true and (.driver_opts // {} | length) == 0)
        and (.networks | keys == ["app","data","edge","vault"])
        and (.networks | to_entries | all(.[]; .key as $key |
            .value | labeled and .name == ($project + "_" + $key) and .external != true))
        and ([.services.web.ports[] | [.host_ip, (.published | tostring), .target, .protocol]]
            == [["127.0.0.1", $port, 8080, "tcp"]])
        and ([.services | del(.web) | .[] | .ports[]?] | length == 0)
        and (.services.server.environment.LGTM_PUBLIC_URL == ("http://127.0.0.1:" + $port))
        and (.services.server.environment.LGTM_WEB_PASSWORD == env.LGTM_WEB_PASSWORD)
        and (.services | [.server, .web] | all(.[]; .build | labeled and .context == $tree))
        and .services.server.image == ("lgtm-server:" + $tag)
        and .services.web.image == ("lgtm-web:" + $tag)
        and (.services.postgres.volumes | any(.type == "volume" and .source == "pgdata"
            and .target == "/var/lib/postgresql/data"))
        and ([.services[].volumes[]?] | all(.[]; if .type == "bind" then
            (.source == $state or (.source | startswith($state + "/") or startswith($tree + "/deploy/")))
            and (.source | contains("/../") or endswith("/..") | not)
            else .type == "volume" and .source == "pgdata" end))
    ' >/dev/null 2>&1
}

s4_container() {
    local service=$1
    jq -e --arg service "$service" --arg run "$s4_run" --arg project "$s4_project"         --arg state "$s4_state" --arg legacy "$s4_legacy" --arg candidate "$s4_candidate"         --arg port "$s4_port" '
        .[0] |
        .Config.Labels as $labels |
        $labels["lgtm.sprint4.run"] == $run
        and $labels["com.docker.compose.project"] == $project
        and $labels["com.docker.compose.service"] == $service
        and ($labels["com.docker.compose.project.working_dir"] |
            . == ($legacy + "/deploy") or . == ($candidate + "/deploy"))
        and ($labels["com.docker.compose.project.config_files"] | split(",")[0] |
            . == ($legacy + "/deploy/compose.yaml") or . == ($candidate + "/deploy/compose.yaml"))
        and (.Mounts | all(.[]; if .Type == "bind" then
            .Source == $state or (.Source | startswith($state + "/")
                or startswith($legacy + "/deploy/") or startswith($candidate + "/deploy/"))
            else .Type == "volume" and .Name == ($project + "_pgdata") end))
        and (if $service == "postgres" or $service == "prepare" then
            any(.Mounts[]; .Type == "volume" and .Name == ($project + "_pgdata")
                and .Destination == "/var/lib/postgresql/data")
            else true end)
        and (if $service == "web" then
            .HostConfig.PortBindings == {"8080/tcp": [{"HostIp": "127.0.0.1", "HostPort": $port}]}
            and (if .State.Running then
                .NetworkSettings.Ports["8080/tcp"] == [{"HostIp": "127.0.0.1", "HostPort": $port}]
                else true end)
            else (.HostConfig.PortBindings // {} | length) == 0 end)
    ' >/dev/null 2>&1
}

s4_record() {
    local mode=$1 type=$2 name=$3 identity=$4 previous
    previous=$(awk -v kind="$type" -v name="$name" '$1 == kind && $2 == name {print $3}' "$s4_dir/objects") ||
        return 1
    if [[ -n $previous && $identity != "$previous" ]]; then
        case "$mode:$type" in
            changed:container|changed:image) ;;
            *) return 1 ;;
        esac
    fi
    printf '%s %s %s\n' "$type" "$name" "$identity"
}

s4_volume() {
    local volume=$1 model
    model=$("$s4_docker" volume inspect "$volume" 2>/dev/null) &&
        jq -e --arg run "$s4_run" --arg project "$s4_project" '
            .[0] | .Driver == "local" and (.Options // {} | length) == 0
            and .Labels["lgtm.sprint4.run"] == $run
            and .Labels["com.docker.compose.project"] == $project
            and .Labels["com.docker.compose.volume"] == "pgdata"
        ' <<<"$model" >/dev/null 2>&1 || return 1
    printf '%s' "$model" | git hash-object --stdin
}

# List only verified objects. A fresh run refuses every collision in its namespace.
s4_scan() {
    local mode=$1 rows owned id name project service model identity network ref image
    local volume=${s4_project}_pgdata
    rows=$("$s4_docker" container ls --all --no-trunc         --format '{{.ID}} {{.Names}} {{.Label "com.docker.compose.project"}} {{.Label "com.docker.compose.service"}}' 2>/dev/null) ||
        return 1
    while read -r id name project service; do
        [[ $project == "$s4_project" || $name == "$s4_project"-* || $name == "$s4_project"_* ]] || continue
        [[ $mode != fresh ]] || return 1
        case $service in
            postgres|openbao|openbao-init|prepare|server|web) ;;
            *) [[ $mode == cleanup ]] && continue; return 1 ;;
        esac
        if model=$("$s4_docker" container inspect "$id" 2>/dev/null) &&
            s4_container "$service" <<<"$model" &&
            s4_record "$mode" container "$name" "$id"; then
            continue
        fi
        [[ $mode == cleanup ]] || return 1
    done <<<"$rows"

    rows=$("$s4_docker" network ls --no-trunc --format '{{.ID}} {{.Name}}' 2>/dev/null) &&
        owned=$("$s4_docker" network ls --no-trunc --filter "label=com.docker.compose.project=$s4_project"             --format '{{.ID}}' 2>/dev/null) || return 1
    while read -r id name; do
        [[ -n $id && ( $name == "$s4_project"_* || $name == "$s4_project"-* ||
            $'\n'$owned$'\n' == *$'\n'"$id"$'\n'* ) ]] || continue
        [[ $mode != fresh ]] || return 1
        network=${name#"$s4_project"_}
        case $network in
            app|data|edge|vault) ;;
            *) [[ $mode == cleanup ]] && continue; return 1 ;;
        esac
        if model=$("$s4_docker" network inspect "$id" 2>/dev/null) &&
            jq -e --arg run "$s4_run" --arg project "$s4_project" --arg network "$network" '
                .[0].Labels | .["lgtm.sprint4.run"] == $run and
                .["com.docker.compose.project"] == $project and .["com.docker.compose.network"] == $network
            ' <<<"$model" >/dev/null 2>&1 &&
            s4_record "$mode" network "$name" "$id"; then
            continue
        fi
        [[ $mode == cleanup ]] || return 1
    done <<<"$rows"

    rows=$("$s4_docker" volume ls --format '{{.Name}}' 2>/dev/null) &&
        owned=$("$s4_docker" volume ls --filter "label=com.docker.compose.project=$s4_project"             --format '{{.Name}}' 2>/dev/null) || return 1
    while read -r name; do
        [[ -n $name && ( $name == "$s4_project"_* || $name == "$s4_project"-* ||
            $'\n'$owned$'\n' == *$'\n'"$name"$'\n'* ) ]] || continue
        [[ $mode != fresh ]] || return 1
        if [[ $name == "$volume" ]] && identity=$(s4_volume "$volume") &&
            s4_record "$mode" volume "$name" "$identity"; then
            continue
        fi
        [[ $mode == cleanup ]] || return 1
    done <<<"$rows"

    rows=$("$s4_docker" image ls --format '{{.Repository}}:{{.Tag}}' 2>/dev/null) || return 1
    for image in lgtm-server lgtm-web; do
        for ref in "$image:$s4_project-3" "$image:$s4_project-4"; do
            [[ $'\n'$rows$'\n' == *$'\n'"$ref"$'\n'* ]] || continue
            [[ $mode != fresh ]] || return 1
            if model=$("$s4_docker" image inspect "$ref" 2>/dev/null) &&
                identity=$(jq -er --arg run "$s4_run" '
                    .[0] | select(.Config.Labels["lgtm.sprint4.run"] == $run) | .Id
                ' <<<"$model" 2>/dev/null) &&
                s4_record "$mode" image "$ref" "$identity"; then
                continue
            fi
            [[ $mode == cleanup ]] || return 1
        done
    done
    return 0
}

s4_binding() {
    local inventory=$1 requirement=$2 running id ports web=
    running=$("$s4_docker" container ls --no-trunc --format '{{.ID}} {{.Ports}}' 2>/dev/null) ||
        return 1
    while read -r id ports; do
        [[ $ports == *":$s4_port->"* ]] || continue
        [[ -z $web && $'\n'$inventory$'\n' == *$'\n'"container $s4_project-web-1 $id"$'\n'* ]] ||
            return 1
        web=$id
    done <<<"$running"
    if [[ -n $web ]]; then
        return 0
    fi
    [[ $requirement != running ]] && s4_socket "$s4_port" >/dev/null 2>&1
}

s4_inventory() {
    local mode=$1 requirement=$2 inventory stores type name identity
    [[ $(s4_daemon) == "$s4_daemon_id" ]] && s4_same_paths &&
        inventory=$(s4_scan "$mode") || return 1
    stores=$(awk '$1 == "volume" || $1 == "network"' "$s4_dir/objects") || return 1
    while read -r type name identity; do
        [[ -z $type || $'\n'$inventory$'\n' == *$'\n'"$type $name $identity"$'\n'* ]] || return 1
    done <<<"$stores"
    if [[ $requirement == running ]]; then
        s4_binding "$inventory" "$requirement"
        return
    fi
    if [[ $mode != changed ]]; then
        s4_binding "$inventory" "$requirement" || return 1
    fi
    (
        umask 077
        local record
        record=$(mktemp "$s4_dir/objects.XXXXXXXX") || exit 1
        if ! printf '%s\n' "$inventory" >"$record" || ! mv -f -- "$record" "$s4_dir/objects"; then
            rm -f -- "$record"
            exit 1
        fi
    ) || return 1
    [[ $mode != changed ]] || s4_binding "$inventory" "$requirement"
}

s4_exit() {
    local status=$1
    if ! sprint4_fixture_close && ((status == 0)); then status=1; fi
    exit "$status"
}

sprint4_fixture_open() {
    set +x
    local source commit root temporary port
    [[ $# == 2 && -n $1 && -n $2 && -z ${s4_dir:-} ]] ||
        { s4_fail 'The fixture requires a source and a candidate commit.'; return 1; }
    source=$(cd -- "$1" && pwd -P) &&
        root=$(s4_git -C "$source" rev-parse --show-toplevel 2>/dev/null) &&
        [[ $root == "$source" ]] &&
        commit=$(s4_git -C "$source" rev-parse --verify --end-of-options "$2^{commit}" 2>/dev/null) ||
        { s4_fail 'The candidate commit could not be resolved.'; return 1; }
    s4_docker=$(command -v docker) && [[ $s4_docker == /* ]] &&
        s4_daemon_id=$(s4_daemon) && [[ -n $s4_daemon_id ]] &&
        temporary=$(cd -- "${TMPDIR:-/tmp}" && pwd -P) &&
        [[ $temporary != *:* && $temporary != *,* && $temporary != *$'\n'* ]] ||
        { s4_fail 'The fixture requires a Docker daemon and a usable temporary directory.'; return 1; }

    s4_dir=$(mktemp -d "${temporary%/}/lgtm-s4.XXXXXXXX") || return 1
    s4_folder_id=$(s4_path_identity "$s4_dir") || { rmdir -- "$s4_dir"; return 1; }
    s4_claimed=no
    trap 's4_exit "$?"' EXIT
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM
    s4_run=$(python3 -c 'import secrets; print(secrets.token_hex(16))') || return 1
    s4_project=lgtm-s4-$s4_run
    s4_commit=$commit
    s4_candidate=$s4_dir/candidate s4_legacy=$s4_dir/legacy s4_state=$s4_dir/state
    S4_REPO_A=$s4_dir/repo-a S4_REPO_B=$s4_dir/repo-b

    port=$(s4_socket 0) && [[ $port != 8787 ]] ||
        { s4_fail 'A private loopback port could not be selected.'; return 1; }
    s4_port=$port
    if ! (
        umask 077
        mkdir -- "$s4_state" &&
            : >"$s4_dir/objects" &&
            s4_overlay >"$s4_dir/ownership.yaml" &&
            python3 -c 'import secrets; print("LGTM_WEB_PASSWORD=" + secrets.token_hex(24))' >"$s4_dir/settings.env"
    ) || ! s4_paths=$(s4_path_identity "$s4_dir" "$s4_state") ||
        ! s4_inventory fresh available; then
        s4_fail 'The private stack and checkout could not be reserved.'
        return 1
    fi
    s4_claimed=yes
    s4_checkouts "$source" >/dev/null 2>&1 && s4_config_id=$(s4_config_identity) ||
        { s4_fail 'The private stack and checkout could not be reserved.'; return 1; }

    S4_FIXTURE_DIR=$s4_dir S4_CANDIDATE_ROOT=$s4_candidate S4_LEGACY_ROOT=$s4_legacy
    export S4_FIXTURE_DIR S4_CANDIDATE_ROOT S4_LEGACY_ROOT S4_REPO_A S4_REPO_B
    unset COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PROJECT_NAME COMPOSE_PROFILES
    unset COMPOSE_DISABLE_ENV_FILE COMPOSE_PATH_SEPARATOR COMPOSE_REMOVE_ORPHANS COMPOSE_IGNORE_ORPHANS
    unset COMPOSE_COMPATIBILITY COMPOSE_CONVERT_WINDOWS_PATHS
    unset LGTM_STACK_STATE_DIR LGTM_STACK_COMPOSE_SNAPSHOT LGTM_IMAGE_TAG LGTM_WEB_PORT LGTM_WEB_PASSWORD
}

sprint4_select() {
    set +x
    local tree tag
    [[ $# == 1 && -n ${s4_dir:-} ]] || { s4_fail 'Select Sprint 3 or Sprint 4 in an open fixture.'; return 1; }
    case $1 in
        3) tree=$s4_legacy ;;
        4) tree=$s4_candidate ;;
        *) s4_fail 'Select Sprint 3 or Sprint 4 in an open fixture.'; return 1 ;;
    esac
    tag=$s4_project-$1
    if [[ ${LGTM_WEB_PORT:-$s4_port} != "$s4_port" ]] ||
        ! s4_same_config ||
        ! s4_source_clean >/dev/null 2>&1 ||
        ! (
            export COMPOSE_FILE="$tree/deploy/compose.yaml:$s4_dir/ownership.yaml" COMPOSE_PATH_SEPARATOR=:
            export COMPOSE_PROJECT_NAME=$s4_project COMPOSE_ENV_FILES=$s4_dir/settings.env
            export LGTM_STACK_STATE_DIR=$s4_state LGTM_IMAGE_TAG=$tag LGTM_WEB_PORT=$s4_port
            unset LGTM_WEB_PASSWORD
            set -o pipefail
            LGTM_WEB_PASSWORD=$(s4_password) && export LGTM_WEB_PASSWORD &&
                s4_model | s4_valid_model "$tree" "$tag"
        ) ||
        ! s4_inventory selected available; then
        s4_fail 'The private stack selection could not be verified.'
        return 1
    fi

    LGTM_WEB_PASSWORD=$(s4_password) || return 1
    export LGTM_WEB_PASSWORD
    export COMPOSE_FILE="$tree/deploy/compose.yaml:$s4_dir/ownership.yaml" COMPOSE_PATH_SEPARATOR=:
    export COMPOSE_PROJECT_NAME=$s4_project COMPOSE_ENV_FILES=$s4_dir/settings.env
    export LGTM_STACK_STATE_DIR=$s4_state LGTM_IMAGE_TAG=$tag LGTM_WEB_PORT=$s4_port
    s4_tree=$tree s4_tag=$tag
}

s4_password() {
    local row
    [[ -O $s4_dir/settings.env && ! -L $s4_dir/settings.env ]] &&
        IFS= read -r row <"$s4_dir/settings.env" &&
        [[ $row =~ ^LGTM_WEB_PASSWORD=[0-9a-f]{48}$ ]] || return 1
    printf '%s\n' "${row#*=}"
}

s4_config_identity() {
    python3 - "$s4_dir/ownership.yaml" "$s4_dir/settings.env" \
        "$s4_legacy/deploy/compose.yaml" "$s4_candidate/deploy/compose.yaml" 2>/dev/null <<'PY'
import hashlib
import os
from pathlib import Path
import stat
import sys

for index, filename in enumerate(sys.argv[1:]):
    path = Path(filename)
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid():
        sys.exit(1)
    if index < 2 and info.st_mode & 0o077:
        sys.exit(1)
    print(hashlib.sha256(path.read_bytes()).hexdigest())
PY
}

s4_same_config() {
    local identity
    identity=$(s4_config_identity) && [[ $identity == "$s4_config_id" ]]
}

sprint4_fixture_close() {
    set +x
    [[ -n ${s4_dir:-} ]] || return 0
    local inventory type name identity current status=0
    [[ -n ${s4_folder_id:-} && $(s4_path_identity "$s4_dir") == "$s4_folder_id" ]] ||
        { s4_fail 'The fixture folder identity changed during cleanup.'; return 1; }
    if [[ ${s4_claimed:-no} == no ]]; then
        :
    elif s4_same_paths && [[ $(s4_daemon) == "$s4_daemon_id" ]]; then
        if inventory=$(s4_scan cleanup); then
            while read -r type name identity; do
                case $type in
                    container) "$s4_docker" container rm -f "$identity" || status=1 ;;
                    network) "$s4_docker" network rm "$identity" || status=1 ;;
                    volume)
                        current=$(s4_volume "$name") &&
                            [[ $current == "$identity" ]] &&
                            "$s4_docker" volume rm "$name" || status=1
                        ;;
                    image)
                        current=$("$s4_docker" image inspect --format '{{.Id}}' "$name") &&
                            [[ $current == "$identity" ]] &&
                            "$s4_docker" image rm "$name" || status=1
                        ;;
                esac
            done <<<"$inventory" >/dev/null 2>&1
        else
            status=1
        fi
        if ((status == 0)) && ! rm -rf -- "$s4_state" 2>/dev/null; then
            s4_remove_state || status=1
        fi
    else
        s4_fail 'The fixture daemon or state identity changed during cleanup.'
        return 1
    fi

    if ((status != 0)) || [[ ! -d $s4_dir || -L $s4_dir || ! -O $s4_dir ]] ||
        ! rm -rf -- "$s4_dir" 2>/dev/null; then
        s4_fail 'The private fixture cleanup could not be completed.'
        return 1
    fi
    unset s4_dir s4_folder_id s4_paths s4_tree s4_tag s4_config_id s4_claimed
    unset S4_FIXTURE_DIR S4_CANDIDATE_ROOT S4_LEGACY_ROOT S4_REPO_A S4_REPO_B
    unset COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PROJECT_NAME LGTM_STACK_STATE_DIR
    unset LGTM_IMAGE_TAG LGTM_WEB_PORT LGTM_WEB_PASSWORD
}

s4_remove_state() {
    local name=$s4_project-state-cleanup image
    image=$(s4_git -C "$s4_candidate" show HEAD:deploy/compose.yaml |
        awk '/^x-postgres-image: &postgres-image / {print $3}') || return 1
    [[ $image == postgres:*@sha256:* ]] && s4_same_paths &&
        [[ $(s4_daemon) == "$s4_daemon_id" ]] &&
        ! "$s4_docker" container inspect "$name" >/dev/null 2>&1 &&
        "$s4_docker" run --rm --pull never --network none --user 0:0 --name "$name"             --label "lgtm.sprint4.run=$s4_run" --mount "type=bind,src=$s4_state,dst=/state"             --entrypoint sh "$image" -ec 'find /state -mindepth 1 -depth -delete' >/dev/null 2>&1
}

s4_selected_settings() {
    [[ -n ${s4_tree:-} &&
        ${COMPOSE_PROJECT_NAME:-} == "$s4_project" &&
        ${COMPOSE_PATH_SEPARATOR:-} == : &&
        ${LGTM_STACK_STATE_DIR:-} == "$s4_state" &&
        ${LGTM_IMAGE_TAG:-} == "$s4_tag" &&
        ${LGTM_WEB_PORT:-} == "$s4_port" &&
        -z ${COMPOSE_PROFILES:-}${COMPOSE_REMOVE_ORPHANS:-}${COMPOSE_IGNORE_ORPHANS:-} &&
        -z ${COMPOSE_COMPATIBILITY:-}${COMPOSE_CONVERT_WINDOWS_PATHS:-} ]] &&
        s4_same_config
}

# The wave 1 password check temporarily removes one server environment value.
# Resolve its override and compare the rest of the model before allowing it.
s4_password_override() (
    set -o pipefail
    local baseline variant settings=$1
    local filter='del(.services.server.environment.LGTM_WEB_PASSWORD, .services.server.restart)'
    baseline=$(COMPOSE_FILE="$settings" s4_model | jq -cS "$filter" 2>/dev/null) &&
        variant=$(s4_model) &&
        jq -e '.services.server | .restart == "no" and
            .environment.LGTM_WEB_PASSWORD == null' <<<"$variant" >/dev/null 2>&1 &&
        variant=$(jq -cS "$filter" <<<"$variant" 2>/dev/null) &&
        [[ $variant == "$baseline" ]]
)

s4_guard_compose() {
    local base=$s4_tree/deploy/compose.yaml settings command arg status=0
    local profiles=()
    settings=$base:$s4_dir/ownership.yaml
    # The database helper passes the base file explicitly. Supply the same
    # ownership overlay as the launcher before it can create the first volume.
    while (($#)); do
        case $1 in
            -f)
                [[ $# -ge 2 && $2 == "$base" ]] || return 1
                shift 2
                ;;
            --profile)
                [[ $# -ge 2 && ( $2 == setup || $2 == '*' ) ]] || return 1
                profiles+=("$1" "$2")
                shift 2
                ;;
            *) break ;;
        esac
    done
    command=${1:-}
    case $command in
        version) "$s4_docker" compose "$@" 2>/dev/null; return ;;
        config) [[ $# == 2 && $2 == --quiet ]] || return 1 ;;
        up|build|run|rm|restart|stop|exec|ps|port|logs) ;;
        *) return 1 ;;
    esac
    for arg in "$@"; do
        case $arg in
            -f|-p*|--file|--file=*|--project-name|--project-name=*|--project-directory|--project-directory=*|\
            --env-file|--env-file=*|--profile|--profile=*|--remove-orphans)
                return 1
                ;;
        esac
    done
    if [[ $command == run ]]; then
        [[ ${2:-} == --rm && ${3:-} == --no-deps && ${4:-} == -T ]] || return 1
        case "${5:-}:${6:-}:${7:-}:$#" in
            openbao-init:::5|prepare:_prepare-db::6|prepare:_seal:[tf]:7) ;;
            *) return 1 ;;
        esac
    fi

    s4_selected_settings || return 1
    export COMPOSE_ENV_FILES=$s4_dir/settings.env
    if [[ $COMPOSE_FILE != "$settings" ]]; then
        [[ $COMPOSE_FILE == "$settings:"* ]] && s4_password_override "$settings" || return 1
    fi

    case $command in
        up|build|run|rm|restart|stop|exec)
            s4_inventory selected available || return 1
            ;;
    esac
    "$s4_docker" compose "${profiles[@]}" "$@" 2>/dev/null || status=$?
    case $command in
        up|build|run|rm|restart|stop)
            # Containers and application images can change. The stores must
            # keep their recorded identities even if the operation fails.
            s4_inventory changed available || return 1
            ;;
    esac
    return "$status"
}

s4_guard_docker() {
    local id
    case ${1:-} in
        compose) shift; s4_guard_compose "$@" ;;
        inspect)
            [[ $# == 4 && $2 == --format && $3 == '{{.State.Health.Status}}' &&
                $4 =~ ^[0-9a-f]{12,64}$ ]] || return 1
            id=$4
            awk -v id="$id" '$1 == "container" && index($3, id) == 1 {found=1} END {exit !found}' +                "$s4_dir/objects" && "$s4_docker" "$@" 2>/dev/null
            ;;
        *) return 1 ;;
    esac
}

s4_guard_curl() {
    local arg target= prefix=http://127.0.0.1:$s4_port
    local args=("$@")
    while (($#)); do
        arg=$1
        shift
        case $arg in
            "$prefix/"*) [[ -z $target ]] || return 1; target=$arg ;;
            -q|--silent|--fail|--no-buffer) ;;
            --cookie|--cookie-jar|--noproxy|--max-time|--header|--data-binary|--write-out)
                (($#)) || return 1
                shift
                ;;
            *) return 1 ;;
        esac
    done
    [[ -n $target ]] && s4_selected_settings && s4_inventory selected running &&
        "$s4_curl" -q "${args[@]}" --noproxy '*' 2>/dev/null
}

# Call this immediately before requests made by a browser or native executable.
sprint4_fixture_ready() {
    set +x
    s4_selected_settings && s4_inventory selected running ||
        s4_fail 'The private website binding could not be verified.'
}

sprint4_fixture_run() (
    set +x
    trap - EXIT HUP INT TERM
    umask 077
    local status=0 tool
    (($#)) && s4_selected_settings && s4_source_clean >/dev/null 2>&1 &&
        s4_inventory selected available ||
        { s4_fail 'The private stack command could not be authorized.'; exit 1; }
    [[ ${COMPOSE_FILE:-} == "$s4_tree/deploy/compose.yaml:$s4_dir/ownership.yaml" &&
        ${COMPOSE_ENV_FILES:-} == "$s4_dir/settings.env" &&
        ${LGTM_WEB_PASSWORD:-} == "$(s4_password)" ]] ||
        { s4_fail 'The private stack command requires the selected settings.'; exit 1; }
    s4_curl=$(command -v curl) && [[ $s4_curl == /* ]] || exit 1
    mkdir -p -- "$s4_dir/bin" || exit 1
    for tool in docker curl; do
        printf '%s\n' '#!/usr/bin/env bash' 'set +x' 'source "$s4_library" || exit 1' \
            "s4_guard_$tool \"\$@\"" >"$s4_dir/bin/$tool" &&
            chmod 0700 "$s4_dir/bin/$tool" || exit 1
    done
    export s4_library s4_dir s4_state s4_paths s4_docker s4_daemon_id s4_curl s4_config_id
    export s4_run s4_project s4_port s4_tree s4_tag s4_legacy s4_candidate
    export BUILDX_CONFIG=$s4_dir/buildx
    unset BUILDX_BUILDER
    PATH="$s4_dir/bin:$PATH" "$@" || status=$?
    s4_source_clean >/dev/null 2>&1 ||
        { s4_fail 'The command changed tracked or untracked candidate source.'; exit 1; }
    exit "$status"
)
