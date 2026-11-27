#!/usr/bin/env bash
set +x
set +a
set -u -o pipefail
umask 077
export LC_ALL=C

# The command guard forwards Docker reads and allows removal of one independently
# verified test volume. It records denied actions without logging their arguments.
case ${0##*/} in
    docker)
        case "${1:-} ${2:-}" in
            'compose '*)
                shift
                args=("$@")
                while (($#)); do
                    case $1 in
                        -f|--file|-p|--project-name|--profile|--env-file|--project-directory|--ansi|--progress)
                            (($# > 1)) || exit 2
                            shift 2
                            ;;
                        --*) shift ;;
                        *) break ;;
                    esac
                done
                case ${1:-} in
                    config|ls|ps|port|version) exec "$S4_TEST_DOCKER" compose "${args[@]}" ;;
                    down)
                        # Only a volume is provisioned. Drop it by its verified identity.
                        down_volumes=false
                        for flag in "$@"; do
                            [[ $flag != -v && $flag != --volumes ]] || down_volumes=true
                        done
                        if $down_volumes && [[ -s $S4_TEST_DIR/owned-volume ]] &&
                            read -r volume fingerprint <"$S4_TEST_DIR/owned-volume" &&
                            model=$("$S4_TEST_DOCKER" compose "${args[@]:0:$(("${#args[@]}" - $#))}" config --format json 2>/dev/null) &&
                            jq -e --arg volume "$volume" '
                                .name == ($volume | sub("_pgdata$"; "")) and
                                ([.volumes[].name] == [$volume])
                            ' <<<"$model" >/dev/null 2>&1 &&
                            [[ $("$S4_TEST_DOCKER" volume inspect "$volume" 2>/dev/null | sha256sum) == "$fingerprint" ]]; then
                            exec "$S4_TEST_DOCKER" volume rm "$volume"
                        fi
                        ;;
                esac
                ;;
            'container ls'|'container inspect'|'volume inspect'|'volume ls'|'network inspect'|'network ls'|\
            'image inspect'|'image ls'|'context inspect'|'context show'|\
            'info '*|'inspect '*|'ps '*|'images '*|'port '*|'version '*|'--version ')
                exec "$S4_TEST_DOCKER" "$@"
                ;;
            'volume rm')
                if [[ -s $S4_TEST_DIR/owned-volume ]] &&
                    read -r volume fingerprint <"$S4_TEST_DIR/owned-volume" &&
                    { [[ $# == 3 && $3 == "$volume" ]] ||
                        [[ $# == 4 && $3 == -f && $4 == "$volume" ]]; } &&
                    [[ $("$S4_TEST_DOCKER" volume inspect "$volume" 2>/dev/null | sha256sum) == "$fingerprint" ]]; then
                    exec "$S4_TEST_DOCKER" "$@"
                fi
                ;;
        esac
        printf '%s\n' 'A Docker lifecycle action was attempted.' >>"$S4_TEST_DIR/actions"
        exit 1
        ;;
    curl|wget)
        printf '%s\n' 'An application request was attempted.' >>"$S4_TEST_DIR/actions"
        exit 1
        ;;
esac

suite_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P) || exit 1
real_docker=$(command -v docker)
if [[ -z $real_docker ]] || ! "$real_docker" info >/dev/null 2>&1 ||
    ! "$real_docker" compose version >/dev/null 2>&1 ||
    ! command -v jq python3 git sha256sum realpath >/dev/null; then
    printf '%s\n' 'The fixture tests require Docker Compose, jq, Python 3, Git and sha256sum.' >&2
    exit 2
fi

work=$(mktemp -d) || exit 1
trap 'rm -rf -- "$work"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
canonical_work=$(cd -- "$work" && pwd -P) || exit 1
work=$canonical_work

mkdir "$work/bin" || exit 2
for name in docker curl wget; do
    ln -s "$suite_root/deploy/test/sprint4-fixture-test.sh" "$work/bin/$name" || exit 2
done
root=$work/source
if ! git clone --quiet --shared --no-checkout -- "$suite_root" "$root" >/dev/null 2>&1 ||
    ! git -C "$root" switch --quiet --detach HEAD >/dev/null 2>&1; then
    printf '%s\n' 'The tests could not create the disposable source checkout.' >&2
    exit 2
fi
export S4_TEST_DOCKER=$real_docker
export PATH="$work/bin:$PATH"
contract=89506b7c
legacy=f6cbea30
previous_freeze=fcfa96c3
commit=$(git -C "$root" rev-parse --verify "$contract^{commit}") || exit 2

# All fixture output is discarded. Assertions report only descriptions.
fixture_call() {
    local status=0
    {
        "$@" || status=$?
        set +x
        set +a
    } >/dev/null 2>&1
    return "$status"
}
fail() { printf '# %s\n' "$1"; return 1; }

docker_identity() {
    "$real_docker" "$1" inspect "$2" 2>/dev/null | sha256sum
}

remove_volume() {
    local entry=$1 volume fingerprint
    [[ -s $entry ]] || return 0
    read -r volume fingerprint <"$entry" || return 1
    if [[ $(docker_identity volume "$volume") == "$fingerprint" ]]; then
        "$real_docker" volume rm "$volume" >/dev/null 2>&1
    fi
}

case_cleanup() {
    local pid command_line container name
    if read -r pid 2>/dev/null <"$S4_TEST_DIR/listener-id" &&
        command_line=$(ps -p "$pid" -o args= 2>/dev/null) &&
        [[ $command_line == python3\ -\ *" $S4_TEST_DIR" ]]; then
        kill "$pid" 2>/dev/null || :
    fi
    if read -r container name 2>/dev/null <"$S4_TEST_DIR/container-id" &&
        [[ $("$real_docker" container inspect --format '{{index .Config.Labels "lgtm.s4.test-owner"}}' \
            "$container" 2>/dev/null) == "$name" ]]; then
        "$real_docker" container rm -f "$container" >/dev/null 2>&1 || :
    fi
    remove_volume "$S4_TEST_DIR/owned-volume" || :
    remove_volume "$S4_TEST_DIR/foreign-volume" || :
    rm -rf -- "$S4_TEST_DIR"
}

in_case() (
    S4_TEST_DIR=$(mktemp -d "$work/case.XXXXXXXX") || return 1
    export S4_TEST_DIR
    trap case_cleanup EXIT
    mkdir "$S4_TEST_DIR/tmp" || return 1
    export TMPDIR="$S4_TEST_DIR/tmp"
    unset COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PROJECT_NAME COMPOSE_PATH_SEPARATOR
    unset COMPOSE_DISABLE_ENV_FILE COMPOSE_PROFILES LGTM_STACK_STATE_DIR LGTM_IMAGE_TAG
    unset LGTM_WEB_PORT LGTM_WEB_PASSWORD
    # A fixture may install its own exit trap. The outer shell retains the test's cleanup.
    (
        fixture_call source "$suite_root/deploy/test/sprint4-fixture.sh" ||
            { fail 'The fixture library must be sourceable.'; return 1; }
        "$@"
    )
)

select_fresh() {
    fixture_call sprint4_fixture_open "$1" "$commit" &&
        fixture_call sprint4_select "${2:-4}" &&
        [[ ${COMPOSE_PROJECT_NAME:-} =~ ^lgtm-s4-[a-z0-9_-]+$ &&
           ${LGTM_WEB_PORT:-} =~ ^[1-9][0-9]{0,4}$ ]] &&
        (( LGTM_WEB_PORT <= 65535 && LGTM_WEB_PORT != 8787 )) &&
        [[ ${S4_FIXTURE_DIR:-} == "$TMPDIR/"* && -d $S4_FIXTURE_DIR &&
           $(realpath -- "$S4_FIXTURE_DIR" 2>/dev/null) == "$S4_FIXTURE_DIR" &&
           ${LGTM_STACK_STATE_DIR:-} == "$S4_FIXTURE_DIR/"* &&
           $(realpath -m -- "$LGTM_STACK_STATE_DIR" 2>/dev/null) == "$LGTM_STACK_STATE_DIR" ]] ||
        fail 'Selection must export a fresh project, state directory and nondefault loopback port.'
}

selected_model() {
    docker compose --profile '*' config --format json 2>/dev/null
}

check_model() {
    local tree=$1 model=$2 settings
    [[ ${COMPOSE_FILE%%"${COMPOSE_PATH_SEPARATOR:-:}"*} == "$tree/deploy/compose.yaml" &&
       -n ${COMPOSE_ENV_FILES:-} && -n ${LGTM_IMAGE_TAG:-} && $LGTM_IMAGE_TAG != local &&
       -n ${LGTM_WEB_PASSWORD:-} ]] || return 1
    local -a files
    IFS=, read -r -a files <<<"$COMPOSE_ENV_FILES"
    for settings in "${files[@]}"; do
        [[ -f $settings && $settings == "$S4_FIXTURE_DIR/"* && -O $settings &&
           $(stat -c '%a' -- "$settings") =~ ^[46]00$ ]] || return 1
    done
    S4_TEST_PASSWORD=$LGTM_WEB_PASSWORD jq -e \
        --arg root "$tree" --arg state "$LGTM_STACK_STATE_DIR" --arg project "$COMPOSE_PROJECT_NAME" \
        --arg port "$LGTM_WEB_PORT" --arg tag "$LGTM_IMAGE_TAG" '
        .name == $project and .volumes.pgdata.name == ($project + "_pgdata")
        and .services.server.image == ("lgtm-server:" + $tag)
        and .services.web.image == ("lgtm-web:" + $tag)
        and .services.server.environment.LGTM_WEB_PASSWORD == env.S4_TEST_PASSWORD
        and .services.server.environment.LGTM_PUBLIC_URL == ("http://127.0.0.1:" + $port)
        and ([.services.web.ports[] | [.host_ip, (.published | tostring), .target, .protocol]]
            == [["127.0.0.1", $port, 8080, "tcp"]])
        and ([.services | del(.web) | .[] | .ports[]?] | length == 0)
        and ([.services[].volumes[]? | select(.type == "bind") | .source] |
            any(. == $state or startswith($state + "/")) and
            all(. == $state or startswith($state + "/") or startswith($root + "/deploy/")))
        and any(.services.postgres.volumes[]; .type == "volume" and .source == "pgdata")
        ' <<<"$model" >/dev/null 2>&1
}

tags_unused() {
    ! "$real_docker" image inspect "lgtm-server:$LGTM_IMAGE_TAG" >/dev/null 2>&1 &&
        ! "$real_docker" image inspect "lgtm-web:$LGTM_IMAGE_TAG" >/dev/null 2>&1
}

without_action() {
    if fixture_call "$@"; then return 1; fi
    [[ ! -s $S4_TEST_DIR/actions ]]
}

missing_selector() {
    without_action sprint4_fixture_open "$root" '' &&
        without_action sprint4_select '' ||
        fail 'A missing selector must be refused before an application request or Docker lifecycle action.'
}

reserved_port() {
    select_fresh "$root" || return 1
    export LGTM_WEB_PORT=8787
    without_action sprint4_select 4 ||
        fail 'A selected port of 8787 must be refused before an application request or Docker lifecycle action.'
}

# A listening socket announces readiness after bind and records any application bytes.
occupied_port() {
    select_fresh "$root" || return 1
    python3 - "$LGTM_WEB_PORT" "$S4_TEST_DIR" >/dev/null 2>&1 <<'PY' &
import pathlib
import socket
import sys

port = int(sys.argv[1])
work = pathlib.Path(sys.argv[2])
with socket.socket() as listener:
    listener.bind(("127.0.0.1", port))
    listener.listen()
    (work / "ready").touch()
    while True:
        conn, _ = listener.accept()
        with conn:
            conn.settimeout(1)
            try:
                if conn.recv(1):
                    (work / "request").touch()
            except TimeoutError:
                pass
PY
    listener_pid=$!
    printf '%s\n' "$listener_pid" >"$S4_TEST_DIR/listener-id"
    local deadline=$((SECONDS + 5))
    until [[ -f $S4_TEST_DIR/ready ]]; do
        if ((SECONDS >= deadline)) || ! kill -0 "$listener_pid" 2>/dev/null; then
            fail 'The occupied-port control must bind to the selected loopback address.'
            return 1
        fi
        sleep 0.05
    done
    without_action sprint4_select 4 &&
        kill -0 "$listener_pid" 2>/dev/null &&
        [[ ! -e $S4_TEST_DIR/request ]] ||
        fail 'An occupied loopback port must be refused before a request or Docker lifecycle action.'
}

foreign_stack() {
    select_fresh "$root" || return 1
    local image=caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0
    local binding fingerprint logs foreign_name foreign_container
    foreign_name=lgtm-s4-test-${S4_TEST_DIR##*.}
    foreign_name=${foreign_name,,}
    foreign_container=$("$real_docker" run -d --pull never --name "$foreign_name" \
        --label "lgtm.s4.test-owner=$foreign_name" --label "com.docker.compose.project=$foreign_name" \
        --label com.docker.compose.service=web --publish "127.0.0.1:$LGTM_WEB_PORT:8080" \
        "$image" caddy respond --listen :8080 --access-log --body fixture 2>/dev/null) ||
        { fail 'The foreign web control needs the pinned Caddy image used by web-testlib.sh.'; return 1; }
    printf '%s %s\n' "$foreign_container" "$foreign_name" >"$S4_TEST_DIR/container-id"
    binding=$("$real_docker" port "$foreign_container" 8080/tcp 2>/dev/null) &&
        [[ $binding == "127.0.0.1:$LGTM_WEB_PORT" ]] ||
        { fail 'The foreign web control must own the selected binding.'; return 1; }
    fingerprint=$(docker_identity container "$foreign_container")
    without_action sprint4_select 4 &&
        [[ $(docker_identity container "$foreign_container") == "$fingerprint" ]] &&
        logs=$("$real_docker" logs "$foreign_container" 2>&1) &&
        ! jq -Re 'fromjson? | select(.request != null)' <<<"$logs" >/dev/null ||
        fail 'The selected port must not send requests or lifecycle actions to the foreign web container.'
}

plant_volume() {
    local entry=$1 volume=$2
    shift 2
    local created fingerprint owner
    owner=${S4_TEST_DIR##*.}
    if "$real_docker" volume inspect "$volume" >/dev/null 2>&1; then return 1; fi
    created=$("$real_docker" volume create --label "lgtm.s4.test-owner=$owner" "$@" "$volume" 2>/dev/null) &&
        [[ $created == "$volume" &&
           $("$real_docker" volume inspect --format '{{index .Labels "lgtm.s4.test-owner"}}' \
              "$volume" 2>/dev/null) == "$owner" ]] || return 1
    fingerprint=$(docker_identity volume "$volume") || return 1
    printf '%s %s\n' "$volume" "$fingerprint" >"$entry"
}

create_foreign_volume() {
    plant_volume "$S4_TEST_DIR/foreign-volume" "$1" \
        --label "com.docker.compose.project=$2" --label com.docker.compose.volume=pgdata
}

volume_unchanged() {
    local volume fingerprint
    read -r volume fingerprint <"$1" &&
        [[ $(docker_identity volume "$volume") == "$fingerprint" ]]
}

stale_volume() {
    select_fresh "$root" || return 1
    create_foreign_volume "${COMPOSE_PROJECT_NAME}_pgdata" "$COMPOSE_PROJECT_NAME" &&
        volume_unchanged "$S4_TEST_DIR/foreign-volume" ||
        { fail 'The stale-volume control must exist with the selected project labels.'; return 1; }
    local refused=false
    if without_action sprint4_select 4; then refused=true; fi
    fixture_call sprint4_fixture_close || :
    "$refused" && volume_unchanged "$S4_TEST_DIR/foreign-volume" &&
        [[ ! -s $S4_TEST_DIR/actions ]] ||
        fail 'A matching Compose project label must not authorize reuse or removal of a stale volume.'
}

source_drift() {
    select_fresh "$root" || return 1
    local changed
    case $1 in
        tracked) changed=$S4_CANDIDATE_ROOT/deploy/README.md ;;
        untracked) changed=$S4_CANDIDATE_ROOT/generation-drift ;;
    esac
    checkout_is_private "$S4_CANDIDATE_ROOT" &&
        [[ $(realpath -m -- "$changed" 2>/dev/null) == "$S4_CANDIDATE_ROOT/"* ]] &&
        printf '%s\n' 'A simulated generation change.' >>"$changed" ||
        { fail 'The generation control must change only the disposable candidate.'; return 1; }
    without_action sprint4_select 4 ||
        fail "Selection must stop before use when generation leaves $1 source drift."
}

s4_private_stack_selection() {
    local check result=0
    for check in missing_selector reserved_port occupied_port foreign_stack stale_volume; do
        in_case "$check" || result=1
    done
    in_case source_drift tracked || result=1
    in_case source_drift untracked || result=1
    return "$result"
}

caller_snapshot() {
    git -C "$1" status --porcelain=v1 --untracked-files=all &&
        sha256sum -- "$1/deploy/README.md" "$1/caller-file" "$1/src/frontend/.cert/local.crt"
}

create_owned_volume() {
    local model=$1 project=$2 key_value
    local -a labels=(--label "com.docker.compose.project=$project" --label com.docker.compose.volume=pgdata)
    jq -e '.volumes.pgdata.labels | to_entries | any(.key | startswith("com.docker.compose.") | not)' \
        <<<"$model" >/dev/null 2>&1 || return 1
    while IFS= read -r key_value; do
        labels+=(--label "$key_value")
    done < <(jq -r '.volumes.pgdata.labels | to_entries[] | "\(.key)=\(.value)"' <<<"$model" 2>/dev/null)
    plant_volume "$S4_TEST_DIR/owned-volume" "${project}_pgdata" "${labels[@]}"
}

run_cutover() {
    local caller=$1 model project state port secret tag fixture
    select_fresh "$caller" 3 ||
        { fail 'The cleanup proof requires an owned revision 3 selection.'; return 1; }
    project=$COMPOSE_PROJECT_NAME state=$LGTM_STACK_STATE_DIR port=$LGTM_WEB_PORT
    secret=$LGTM_WEB_PASSWORD tag=$LGTM_IMAGE_TAG fixture=$S4_FIXTURE_DIR
    model=$(selected_model) && check_model "$S4_LEGACY_ROOT" "$model" && tags_unused ||
        { fail 'Revision 3 must resolve the archived base, private credentials, mounts and binding.'; return 1; }
    create_owned_volume "$model" "$project" ||
        { fail 'The published model must provide a run label for the PostgreSQL volume.'; return 1; }

    fixture_call sprint4_select 4 && model=$(selected_model) && check_model "$S4_CANDIDATE_ROOT" "$model" &&
        tags_unused &&
        [[ $COMPOSE_PROJECT_NAME == "$project" && $LGTM_STACK_STATE_DIR == "$state" &&
           $LGTM_WEB_PORT == "$port" && $LGTM_WEB_PASSWORD == "$secret" && $LGTM_IMAGE_TAG != "$tag" ]] &&
        volume_unchanged "$S4_TEST_DIR/owned-volume" ||
        { fail 'Cutover must change the image tag and preserve the project, volume, state, port and password.'; return 1; }

    fixture_call sprint4_fixture_close &&
        [[ ! -e $fixture && ! -s $S4_TEST_DIR/actions ]] &&
        ! "$real_docker" volume inspect "${project}_pgdata" >/dev/null 2>&1 ||
        { fail 'Cleanup must remove the run directory and the verified owned PostgreSQL volume.'; return 1; }

    printf '%s\n' "$project" "$state" "$tag" >"$S4_TEST_DIR/cutover"
}

owned_cleanup_case() {
    local caller=$S4_TEST_DIR/caller before after other first second first_state second_state first_tag second_tag
    git clone --quiet --shared --no-checkout -- "$root" "$caller" >/dev/null 2>&1 &&
        git -C "$caller" switch --quiet --detach HEAD >/dev/null 2>&1 &&
        mkdir -p "$caller/src/frontend/.cert" &&
        printf '%s\n' 'A tracked caller change.' >>"$caller/deploy/README.md" &&
        printf '%s\n' 'An untracked caller file.' >"$caller/caller-file" &&
        printf '%s\n' 'A local certificate marker.' >"$caller/src/frontend/.cert/local.crt" &&
        before=$(caller_snapshot "$caller") ||
        { fail 'The caller snapshot must contain the tracked, untracked and ignored controls.'; return 1; }
    other=lgtm-s4-unrelated-${S4_TEST_DIR##*.}
    other=${other,,}
    create_foreign_volume "${other}_pgdata" "$other" ||
        { fail 'The unrelated volume control must exist before the first run.'; return 1; }

    # Use conflicting caller settings to verify a fresh open owns its selectors.
    export COMPOSE_FILE=$caller/deploy/compose.yaml COMPOSE_ENV_FILES=$caller/caller-file
    export COMPOSE_PROJECT_NAME=$other COMPOSE_PROFILES=foreign COMPOSE_PATH_SEPARATOR=';'
    export COMPOSE_DISABLE_ENV_FILE=1 LGTM_STACK_STATE_DIR=$caller
    export LGTM_IMAGE_TAG=local LGTM_WEB_PORT=4321 LGTM_WEB_PASSWORD=fixture-caller-marker
    run_cutover "$caller" || return 1
    { read -r first && read -r first_state && read -r first_tag; } <"$S4_TEST_DIR/cutover" || return 1
    after=$(caller_snapshot "$caller") &&
        [[ $before == "$after" ]] && volume_unchanged "$S4_TEST_DIR/foreign-volume" ||
        { fail 'The first run must preserve the unrelated volume, caller files and local certificate.'; return 1; }

    run_cutover "$caller" || return 1
    { read -r second && read -r second_state && read -r second_tag; } <"$S4_TEST_DIR/cutover" || return 1
    after=$(caller_snapshot "$caller") &&
        [[ $first != "$second" && $first_state != "$second_state" &&
           $first_tag != "$second_tag" && $before == "$after" ]] &&
        volume_unchanged "$S4_TEST_DIR/foreign-volume" ||
        fail 'A second run must claim a fresh identity and preserve the caller and unrelated volume again.'
}

s4_owned_cleanup() { in_case owned_cleanup_case; }

checkout_is_private() {
    local path=$1 canonical top
    [[ -d $path ]] &&
        canonical=$(cd -- "$path" && pwd -P) &&
        [[ $canonical == "$S4_FIXTURE_DIR/"* ]] &&
        top=$(git -C "$canonical" rev-parse --show-toplevel 2>/dev/null) &&
        [[ $top == "$canonical" ]]
}

git_workspaces_case() {
    local actual status legacy_root branch branch_tip base_tip origin work_a work_b dirty
    fixture_call sprint4_fixture_open "$root" "$commit" &&
        [[ ${S4_FIXTURE_DIR:-} == "$TMPDIR/"* && -d $S4_FIXTURE_DIR ]] &&
        checkout_is_private "${S4_CANDIDATE_ROOT:-}" &&
        actual=$(git -C "$S4_CANDIDATE_ROOT" rev-parse HEAD 2>/dev/null) &&
        status=$(git -C "$S4_CANDIDATE_ROOT" status --porcelain=v1 --untracked-files=all 2>/dev/null) &&
        [[ $actual == "$commit" && -z $status ]] &&
        git -C "$S4_CANDIDATE_ROOT" cat-file -e "$previous_freeze^{commit}" 2>/dev/null &&
        git -C "$S4_CANDIDATE_ROOT" cat-file -e "$contract^{commit}" 2>/dev/null &&
        git -C "$S4_CANDIDATE_ROOT" cat-file -e "$legacy^{commit}" 2>/dev/null ||
        { fail 'The private candidate checkout must be clean at the exact commit and retain both freeze objects and the legacy revision.'; return 1; }

    legacy_root=$(cd -- "${S4_LEGACY_ROOT:-/dev/null}" 2>/dev/null && pwd -P) &&
        [[ $legacy_root == "$S4_FIXTURE_DIR/"* && ! -e $legacy_root/.git ]] &&
        git -C "$root" show "$legacy:src/contracts/proto/lgtm/link/v1/link.proto" 2>/dev/null |
        cmp -s - "$legacy_root/src/contracts/proto/lgtm/link/v1/link.proto" ||
        { fail 'The private legacy tree must contain the pinned revision 1 archive.'; return 1; }

    checkout_is_private "${S4_REPO_A:-}" && checkout_is_private "${S4_REPO_B:-}" &&
        work_a=$(cd -- "$S4_REPO_A" && pwd -P) && work_b=$(cd -- "$S4_REPO_B" && pwd -P) &&
        [[ $work_a != "$work_b" && $work_a != "$S4_CANDIDATE_ROOT" && $work_b != "$S4_CANDIDATE_ROOT" ]] ||
        { fail 'The two repository workspaces must have distinct private Git roots.'; return 1; }

    local checkout base
    for checkout in "$work_a" "$work_b"; do
        origin=$(git -C "$checkout" config --get remote.origin.url 2>/dev/null) || continue
        origin=${origin#file://}
        [[ $origin == /* ]] || origin=$checkout/$origin
        origin=$(cd -- "$origin" 2>/dev/null && pwd -P) || continue
        [[ $origin == "$S4_FIXTURE_DIR/"* && $origin != "$work_a" && $origin != "$work_b" ]] || continue
        branch=$(git -C "$checkout" symbolic-ref --quiet --short HEAD 2>/dev/null) || continue
        base=$(git -C "$origin" symbolic-ref --quiet --short HEAD 2>/dev/null) || continue
        branch_tip=$(git -C "$checkout" rev-parse HEAD 2>/dev/null) || continue
        base_tip=$(git -C "$origin" rev-parse "refs/heads/$base" 2>/dev/null) ||
            base_tip=$(git -C "$checkout" rev-parse "refs/heads/$base" 2>/dev/null) || continue
        dirty=$(git -C "$checkout" status --porcelain=v1 --untracked-files=all 2>/dev/null) || continue
        if [[ $branch != "$base" && $branch_tip != "$base_tip" && -n $dirty ]] &&
            git -C "$checkout" merge-base "$branch_tip" "$base_tip" >/dev/null 2>&1; then
            return 0
        fi
    done
    fail 'One workspace must have its own local origin, a divergent branch and an uncommitted change.'
}

s4_two_git_workspaces() { in_case git_workspaces_case; }

tests=(s4_private_stack_selection s4_owned_cleanup s4_two_git_workspaces)
if [[ $# == 2 && $1 == --only ]]; then
    case $2 in
        s4_private_stack_selection|s4_owned_cleanup|s4_two_git_workspaces) tests=("$2") ;;
        *) exit 2 ;;
    esac
elif (($#)); then
    exit 2
fi
printf '1..%s\n' "${#tests[@]}"
status=0 index=0
for test_name in "${tests[@]}"; do
    index=$((index + 1))
    if "$test_name"; then
        printf 'ok %s - %s\n' "$index" "$test_name"
    else
        printf 'not ok %s - %s\n' "$index" "$test_name"
        status=1
    fi
done
exit "$status"
