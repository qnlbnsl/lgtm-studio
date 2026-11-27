#!/usr/bin/env bash
# Every failed step in a fixture command chain reports the same observation.
# shellcheck disable=SC2015
set +x
set -u -o pipefail
umask 077
export LC_ALL=C

cd -- "${BASH_SOURCE[0]%/*}/../.." || exit 2

client_package=./src/client/cmd/lgtm-client
browser_probe=deploy/test/checks-s4-browser.mjs
project_service=lgtm.v1.ProjectService

# R3, R5 through R10, and the receipt and buffer part of R11 belong to the in-process Go tests.
# R14 is the owner's check on the deployed stack.

fail() {
    printf '%s\n' "$1"
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is missing."
}

rpc() {
    local procedure=$1 data=$2 response
    shift 2
    local -a cookies=()
    [[ ! -f $jar ]] || cookies=(--cookie "$jar")
    response=$(curl -q --silent --noproxy '*' --max-time 32 "${cookies[@]}" "$@" \
        --header 'Content-Type: application/json' --header 'Connect-Protocol-Version: 1' \
        --data-binary @- --write-out $'\n%{http_code}' "$web_url/lgtm.v1.$procedure" <<< "$data") ||
        return 1
    http_code=${response##*$'\n'}
    body=${response%$'\n'*}
}

pause() {
    local timer
    exec {timer}<> <(:)
    read -r -t 1 -u "$timer" _ || :
    exec {timer}>&-
}

close_watch() {
    if [[ -n $watch_fd ]]; then
        exec {watch_fd}<&-
        kill "$watch_pid" 2>/dev/null || :
        wait "$watch_pid" 2>/dev/null || :
        watch_fd=''
    fi
}

# The check shell calls this function from its exit trap.
# shellcheck disable=SC2317
cleanup() {
    local status=$? pid id data
    trap - EXIT
    close_watch
    for pid in "${fixture_pids[@]}"; do
        # A suspended client also exits on this signal.
        kill -KILL "$pid" 2>/dev/null || :
        wait "$pid" 2>/dev/null || :
    done
    if [[ -f $jar ]]; then
        for id in "${fixture_ids[@]}"; do
            data=$(jq -nc --arg id "$id" '{id: $id}')
            rpc ClientService/RemoveClient "$data" >/dev/null 2>&1 || :
        done
        for id in "${fixture_tokens[@]}"; do
            data=$(jq -nc --arg id "$id" '{tokenId: $id}')
            rpc ClientService/RevokeClientToken "$data" >/dev/null 2>&1 || :
        done
        rpc SessionService/SignOut '{}' >/dev/null 2>&1 || :
    fi
    if [[ -n $scratch ]]; then
        rm -rf -- "$scratch" || status=1
    fi
    exit "$status"
}

web_session() {
    local port=${LGTM_WEB_PORT:-} address data
    need curl
    need jq
    if [[ -z $port ]]; then
        need docker
        local -a files=()
        [[ -n ${COMPOSE_FILE:-} ]] || files=(-f deploy/compose.yaml)
        address=$(docker compose "${files[@]}" port web 8080 2>/dev/null) ||
            fail 'Compose could not report the published website port.'
        port=${address##*:}
    fi
    [[ $port =~ ^[0-9]{1,5}$ ]] && ((10#$port >= 1 && 10#$port <= 65535)) ||
        fail 'LGTM_WEB_PORT is outside the valid port range.'
    web_url="http://127.0.0.1:$((10#$port))"
    [[ -n ${LGTM_WEB_PASSWORD:-} ]] || fail 'LGTM_WEB_PASSWORD is missing.'
    export LGTM_WEB_PASSWORD
    scratch=$(mktemp -d "${TMPDIR:-/tmp}/lgtm-s4-check.XXXXXXXX") ||
        fail 'A temporary fixture directory could not be created.'
    scratch=$(cd -- "$scratch" && pwd -P) || fail 'The fixture directory could not be resolved.'
    jar=$scratch/session.cookies
    data=$(printf '%s' "$LGTM_WEB_PASSWORD" | jq -Rsc '{password: .}') ||
        fail 'The sign-in request could not be prepared.'
    rpc SessionService/SignIn "$data" --cookie-jar "$jar" && [[ $http_code == 200 ]] &&
        jq -e '.result == "SIGN_IN_RESULT_SIGNED_IN" or .result == 1' <<< "$body" >/dev/null ||
        fail 'The configured password did not sign in to the stack.'
}

require_projects() {
    local procedure=$1
    rpc SessionService/GetServer '{}' && [[ $http_code == 200 ]] &&
        jq -e '.signedIn == true and .product == "lgtm-studio" and .sample != true' <<< "$body" >/dev/null ||
        fail 'GetServer did not report a signed-in real server.'
    jq -e --arg service "$project_service" '(.services // []) | index($service) != null' \
        <<< "$body" >/dev/null || fail "GetServer did not advertise $project_service for $procedure."
}

# Connect puts a five-byte header before each uncompressed streaming message.
open_watch() {
    close_watch
    exec {watch_fd}< <(
        exec curl -q --silent --noproxy '*' --max-time 240 --no-buffer --cookie "$jar" \
            --header 'Content-Type: application/connect+json' \
            --header 'Connect-Protocol-Version: 1' --data-binary @- \
            "$web_url/lgtm.v1.$1" < <(printf '\000\000\000\000\002{}')
    )
    watch_pid=$!
    read_frame "$((SECONDS + 5))" && jq -e '.all | type == "object"' <<< "$frame" >/dev/null
}

read_frame() {
    local deadline=$1 flags=0 size=0 octet byte i remaining
    for ((i = 0; i < 5; i++)); do
        remaining=$((deadline - SECONDS))
        ((remaining > 0)) || return 1
        IFS= read -r -d '' -n 1 -t "$remaining" -u "$watch_fd" octet || return 1
        byte=0
        [[ -z $octet ]] || printf -v byte '%d' "'$octet"
        if ((i == 0)); then
            flags=$byte
        else
            size=$((size * 256 + byte))
        fi
    done
    remaining=$((deadline - SECONDS))
    ((flags == 0 && size > 0 && size <= 16777216 && remaining > 0)) || return 1
    IFS= read -r -N "$size" -t "$remaining" -u "$watch_fd" frame
}

wait_client() {
    local deadline=$((SECONDS + $1)) predicate=$2
    shift 2
    while read_frame "$deadline"; do
        if record=$(jq -ce "$@" ".saved | select($predicate)" <<< "$frame" 2>/dev/null); then
            return 0
        fi
    done
    return 1
}

build_client() {
    need go
    client_bin=$scratch/lgtm-client
    go build -o "$client_bin" "$client_package" >"$scratch/build.log" 2>&1 ||
        fail 'The current lgtm-client command did not build.'
}

enroll_machine() {
    local label=$1 token token_id data
    need timeout
    state_home=$scratch/state-$label
    rpc ClientService/CreateClientToken '{}' && [[ $http_code == 200 ]] ||
        fail 'CreateClientToken did not issue a join token.'
    token_id=$(jq -er '.tokenId | strings | select(length > 0)' <<< "$body") ||
        fail 'CreateClientToken did not return a token identifier.'
    fixture_tokens+=("$token_id")
    token=$(jq -er '.token | strings | select(length > 0)' <<< "$body") ||
        fail 'CreateClientToken did not return a join token.'

    open_watch ClientService/WatchClients || fail 'WatchClients did not return its first machine list.'
    timeout --kill-after=2s 15s env LGTM_CLIENT_STATE_DIR="$state_home/lgtm-client" \
        "$client_bin" join "$web_url" "$token" >"$scratch/join.log" 2>&1 ||
        fail 'The fixture client did not join with the fresh token within 15 seconds.'
    # The dollar sign belongs to the jq predicate.
    # shellcheck disable=SC2016
    wait_client 15 '(.state == "CLIENT_STATE_WAITING_FOR_APPROVAL" or .state == 1) and .tokenId == $token' \
        --arg token "$token_id" || fail 'WatchClients did not show the joined machine waiting for approval.'
    machine_id=$(jq -er '.id | strings | select(length > 0)' <<< "$record") ||
        fail 'WatchClients did not report an identifier for the waiting machine.'
    fixture_ids+=("$machine_id")
    data=$(jq -nc --arg id "$machine_id" --arg name "Sprint 4 $label $machine_id" '{id: $id, name: $name}')
    rpc ClientService/ApproveClient "$data" && [[ $http_code == 200 ]] ||
        fail 'ApproveClient did not approve the waiting machine.'
}

start_client() {
    client_log=$scratch/client-$1.log
    LGTM_CLIENT_STATE_DIR=$state_home/lgtm-client "$client_bin" run >"$client_log" 2>&1 &
    client_pid=$!
    fixture_pids+=("$client_pid")
}

running_machine() {
    enroll_machine "$1"
    start_client "$1"
    # The dollar sign belongs to the jq predicate.
    # shellcheck disable=SC2016
    wait_client 35 '.id == $id and (.state == "CLIENT_STATE_ONLINE" or .state == 3)' --arg id "$machine_id" ||
        fail 'WatchClients did not show the approved fixture machine online.'
    close_watch
}

# The loopback origins are Git metadata. No fixture contacts a remote.
origin_repos=http://127.0.0.1/sprint4/original.git
origin_other=http://127.0.0.1/sprint4/other.git

make_repository() {
    local folder=$1
    mkdir -p -- "$folder/subfolder" &&
        git -C "$folder" init --quiet --initial-branch=main &&
        printf '%s\n' 'Sprint 4 fixture.' >"$folder/fixture.txt" &&
        git -C "$folder" add fixture.txt &&
        git -C "$folder" -c user.name='Sprint Four' -c user.email='fixture@localhost' \
            -c core.hooksPath=/dev/null -c commit.gpgSign=false commit --quiet -m 'Add the fixture.' &&
        git -C "$folder" remote add origin "$origin_repos" ||
        fail 'The Git repository fixture could not be created.'
}

make_checkout() {
    local original=$1 folder=$2 remote=$3
    git -c core.hooksPath=/dev/null clone --quiet --local -- "$original" "$folder" &&
        git -C "$folder" remote set-url origin "$remote" ||
        fail 'The second Git checkout could not be created.'
}

read_repositories() {
    open_watch ProjectService/WatchRepositories ||
        fail 'WatchRepositories did not return its first repository list.'
    repositories=$(jq -cSe '
        .all | select(type == "object") |
        (.repositories // []) | map(.folders = ((.folders // []) | sort_by(.id))) | sort_by(.id)
    ' <<< "$frame") || fail 'The WatchRepositories snapshot did not contain a repository list.'
    close_watch
}

folder_call() {
    local procedure=$1 client=$2 path=$3 key=$4 target=${5:-} data
    data=$(jq -nc --arg client "$client" --arg path "$path" --arg target "$target" \
        '{clientId: $client, path: $path} + (if $target == "" then {} else {repositoryId: $target} end)') ||
        return 1
    rpc "ProjectService/$procedure" "$data" --header "Lgtm-Request-Key: $key"
}

add_folder() {
    local client=$1 typed=$2 resolved=$3 key=$4 target=${5:-}
    folder_call AddRepository "$client" "$typed" "$key" "$target" && [[ $http_code == 200 ]] ||
        fail 'AddRepository did not return a saved repository and folder.'
    jq -e --arg client "$client" --arg resolved "$resolved" --arg target "$target" '
        . as $reply |
        (.repository.id | type == "string" and length > 0) and
        ($target == "" or .repository.id == $target) and
        (.folder.id | type == "string" and length > 0) and
        .folder.clientId == $client and .folder.resolvedPath == $resolved and
        ([.repository.folders[]? | select(. == $reply.folder)] | length == 1)
    ' <<< "$body" >/dev/null || fail 'AddRepository did not return the chosen machine and resolved path under its saved repository.'
    saved_reply=$(jq -c . <<< "$body")
    repository_id=$(jq -r '.repository.id' <<< "$body")
}

refused() {
    [[ $http_code == "$1" ]] &&
        jq -e --arg code "$2" --arg message "$3" '.code == $code and .message == $message' <<< "$body" >/dev/null
}

check_r1() {
    local folder first before
    web_session
    require_projects 'InspectFolder and WatchRepositories'
    need git
    build_client
    running_machine primary
    folder=$scratch/inspect
    make_repository "$folder"
    git -C "$folder" update-ref refs/remotes/origin/main HEAD &&
        git -C "$folder" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main &&
        git -C "$folder" switch --quiet -c feature &&
        printf '%s\n' 'An uncommitted change.' >>"$folder/fixture.txt" ||
        fail 'The dirty checkout and its remote head could not be prepared.'
    first=$(git -C "$folder" rev-parse HEAD) || fail 'Git did not report the first commit.'
    read_repositories
    before=$repositories
    folder_call InspectFolder "$machine_id" "$folder/subfolder" s4-inspect && [[ $http_code == 200 ]] ||
        fail 'InspectFolder did not return folder facts through the running client.'
    jq -e --arg path "$folder" --arg first "$first" --arg remote "$origin_repos" '
        .facts | .resolvedPath == $path and .name == "inspect" and .mainBranch == "main" and
        .checkedOutBranch == "feature" and .remote == $remote and .firstCommit == $first and
        (.clean // false) == false and (.remoteOmitted // false) == false
    ' <<< "$body" >/dev/null || fail 'InspectFolder did not report the resolved, dirty checkout and its two branches, origin, and first commit.'
    read_repositories
    [[ $repositories == "$before" ]] || fail 'The repository or folder list changed after InspectFolder.'
    printf '%s\n' 'InspectFolder returned the dirty checkout facts and left the repository and folder list unchanged.'
}

check_r2() {
    local folder before after
    web_session
    require_projects 'AddRepository and WatchRepositories'
    need git
    build_client
    running_machine primary
    folder=$scratch/add
    make_repository "$folder"
    ln -s -- "$folder" "$scratch/typed" && mkdir -- "$scratch/plain" ||
        fail 'The linked path and the folder outside Git could not be prepared.'
    read_repositories
    before=$repositories
    add_folder "$machine_id" "$scratch/typed/subfolder" "$folder" s4-add
    read_repositories
    after=$repositories
    jq -e --argjson before "$before" --argjson reply "$saved_reply" '
        ([.[] | select(.id == $reply.repository.id)] | length == 1) and
        any(.[]; .id == $reply.repository.id and .name == "add" and .folders == [$reply.folder]) and
        [.[] | select(.id != $reply.repository.id)] == $before
    ' <<< "$after" >/dev/null || fail 'WatchRepositories did not show exactly the new repository and its resolved folder.'
    folder_call AddRepository "$machine_id" "$scratch/plain" s4-plain &&
        refused 400 failed_precondition 'The folder is not a Git repository.' ||
        fail 'AddRepository did not return the fixed reason for a folder outside Git.'
    read_repositories
    [[ $repositories == "$after" ]] || fail 'The repository or folder list changed after AddRepository refused the plain folder.'
    printf '%s\n' 'AddRepository saved the resolved folder and refused a plain folder without saving anything else.'
}

check_r4() {
    local primary target folder other mismatch first before after
    web_session
    require_projects 'AddRepository on a second machine'
    need git
    build_client
    running_machine primary
    primary=$machine_id
    running_machine secondary
    [[ $primary != "$machine_id" ]] || fail 'The two clients shared one machine identifier.'
    folder=$scratch/original
    other=$scratch/other-name
    mismatch=$scratch/different-origin
    make_repository "$folder"
    make_checkout "$folder" "$other" "$origin_repos"
    make_checkout "$folder" "$mismatch" "$origin_other"
    first=$(git -C "$folder" rev-parse HEAD) &&
        [[ $(git -C "$mismatch" rev-parse HEAD) == "$first" ]] ||
        fail 'The different-origin checkout did not share the original first commit.'
    read_repositories
    before=$repositories
    add_folder "$primary" "$folder" "$folder" s4-original
    target=$repository_id
    add_folder "$machine_id" "$other" "$other" s4-second "$target"
    read_repositories
    after=$repositories
    jq -e --argjson before "$before" --arg id "$target" --arg first "$primary" --arg second "$machine_id" \
        --arg original "$folder" --arg other "$other" '
        any(.[]; .id == $id and .name == "original" and (.folders | length == 2) and
            any(.folders[]; .clientId == $first and .resolvedPath == $original) and
            any(.folders[]; .clientId == $second and .resolvedPath == $other)) and
        [.[] | select(.id != $id)] == $before
    ' <<< "$after" >/dev/null || fail 'WatchRepositories did not show the two machine folders on one repository.'
    folder_call AddRepository "$machine_id" "$mismatch" s4-other-origin "$target" &&
        refused 400 failed_precondition 'The folder does not belong to the selected repository.' ||
        fail 'AddRepository did not refuse the different origin that shared the first commit.'
    read_repositories
    [[ $repositories == "$after" ]] || fail 'The repository or folder list changed after the different origin was refused.'
    printf '%s\n' 'The second machine attached its differently named checkout and was refused for the different origin.'
}

check_r11() {
    local commit deadline
    [[ -n ${LGTM_REV1_COMMIT:-} ]] || fail 'LGTM_REV1_COMMIT is unset or empty.'
    need git
    need go
    need tar
    commit=$(git rev-parse --verify --end-of-options "${LGTM_REV1_COMMIT}^{commit}") ||
        fail 'LGTM_REV1_COMMIT did not resolve to a local commit.'
    web_session
    mkdir -- "$scratch/revision-1" && git archive "$commit" | tar -x -C "$scratch/revision-1" ||
        fail 'Git could not extract the revision 1 commit into the temporary directory.'
    grep -Eq '^const linkRevision = 1$' "$scratch/revision-1/src/client/internal/link/conn.go" ||
        fail 'The archived client does not declare link revision 1.'
    client_bin=$scratch/lgtm-client-revision-1
    (cd -- "$scratch/revision-1" && go build -buildvcs=false -o "$client_bin" "$client_package") >"$scratch/build.log" 2>&1 ||
        fail 'The archived revision 1 client did not build.'
    enroll_machine revision-1
    start_client revision-1
    deadline=$((SECONDS + 20))
    while :; do
        if jq -Ren 'any(inputs | fromjson?; .msg == "link_refused" and .code == "link_revision_mismatch")' \
            "$client_log" >/dev/null; then
            break
        fi
        ((SECONDS < deadline)) && kill -0 "$client_pid" 2>/dev/null ||
            fail 'The revision 1 client did not receive link_revision_mismatch from the server.'
        pause
    done
    open_watch ClientService/WatchClients &&
        jq -e --arg id "$machine_id" 'any(.all.clients[]?; .id == $id and
            .state != "CLIENT_STATE_ONLINE" and .state != 3 and (.connectedAt // "") == "")' <<< "$frame" >/dev/null ||
        fail 'WatchClients did not show that the refused revision 1 machine stayed disconnected.'
    printf '%s\n' 'The server refused the revision 1 client with link_revision_mismatch.'
}

wait_file() {
    local path=$1 pid=$2 deadline=$((SECONDS + $3))
    until [[ -f $path ]]; do
        ((SECONDS < deadline)) && kill -0 "$pid" 2>/dev/null || return 1
        pause
    done
}

check_r12() {
    local primary primary_pid secondary orphan shared browser_pid request_pid
    local folder shared_folder other data key deadline
    web_session
    require_projects 'WatchRepositories and open folder operations on machine removal'
    need node
    need git
    build_client
    running_machine removed
    primary=$machine_id primary_pid=$client_pid
    running_machine retained
    secondary=$machine_id

    folder=$scratch/orphan
    shared_folder=$scratch/shared
    other=$scratch/retained
    make_repository "$folder"
    make_repository "$shared_folder"
    make_checkout "$shared_folder" "$other" "$origin_repos"
    add_folder "$primary" "$folder" "$folder" s4-orphan
    orphan=$repository_id
    add_folder "$primary" "$shared_folder" "$shared_folder" s4-shared
    shared=$repository_id
    add_folder "$secondary" "$other" "$other" s4-retained "$shared"

    node "$browser_probe" removal "$web_url" "$scratch" "$orphan" "$shared" \
        "$primary" "$secondary" "$folder" "$shared_folder" "$other" >"$scratch/browser.log" 2>&1 &
    browser_pid=$!
    fixture_pids+=("$browser_pid")
    wait_file "$scratch/page-ready" "$browser_pid" 45 ||
        fail 'The open website did not show both repositories and the folders from both machines before removal.'

    kill -STOP "$primary_pid" 2>/dev/null || fail 'The fixture client could not be paused for the open operation.'
    key=s4-remove-open
    (
        if folder_call InspectFolder "$primary" "$folder" "$key" &&
            refused 404 not_found 'The machine was removed.'; then
            : > "$scratch/operation-failed"
        fi
    ) &
    request_pid=$!
    fixture_pids+=("$request_pid")
    # A conflicting request proves the original request claimed its key while still waiting.
    pause
    deadline=$((SECONDS + 12))
    folder_call InspectFolder "$primary" "$shared_folder" "$key" &&
        ((SECONDS < deadline)) &&
        refused 409 already_exists 'That request key was already used for a different request.' &&
        kill -0 "$request_pid" 2>/dev/null ||
        fail 'The original folder request did not claim its key and remain open before removal.'
    data=$(jq -nc --arg id "$primary" '{id: $id}')
    rpc ClientService/RemoveClient "$data" && [[ $http_code == 200 ]] ||
        fail 'RemoveClient did not remove the chosen machine.'
    wait "$request_pid" 2>/dev/null || :
    [[ -f $scratch/operation-failed ]] ||
        fail 'The open folder request did not fail with the fixed machine-removed reason.'
    wait "$browser_pid" || fail 'The open website did not lose the folders from the removed machine and orphaned repository while keeping the other folder.'
    printf '%s\n' 'The open page dropped the removed folders and orphaned repository, kept the other folder, and the open operation failed.'
}

check_r13() {
    local status
    web_session
    need node
    node "$browser_probe" modes "$web_url" "$scratch" >"$scratch/browser.log" 2>&1
    status=$?
    # The browser helper keeps its diagnostics private. Its exit code selects a fixed observation.
    case $status in
        0) printf '%s\n' 'One build kept sample projects in both sample modes and loaded real repositories without sample projects.' ;;
        20) fail 'The unpinned website build for the three browser modes could not be created.' ;;
        21) fail 'Playwright or its Chromium browser could not start.' ;;
        22) fail 'The loopback mock server did not start.' ;;
        23) fail 'The website did not show sample projects and the expected in-page sample calls.' ;;
        24) fail 'The website did not show sample projects and the expected mock server calls.' ;;
        25) fail "The real GetServer reply did not advertise $project_service at website contract revision 1." ;;
        26) fail 'The real website did not finish both watches or still held sample projects.' ;;
        *) fail 'The browser probe could not observe the three website modes.' ;;
    esac
}

run_check() {
    local number=$1 observation
    if observation=$(
        scratch='' jar='' web_url='' watch_fd='' watch_pid=''
        fixture_pids=() fixture_ids=() fixture_tokens=()
        trap cleanup EXIT
        trap 'exit 1' HUP INT TERM
        {
            case $number in
                1) check_r1 ;;
                2) check_r2 ;;
                4) check_r4 ;;
                11) check_r11 ;;
                12) check_r12 ;;
                13) check_r13 ;;
            esac
        } 2>/dev/null
    ); then
        printf 'ok %s - R%s %s\n' "$number" "$number" "$observation"
    else
        [[ -n $observation ]] || observation='The fixture ended before it recorded an observation.'
        printf 'not ok %s - R%s %s\n' "$number" "$number" "${observation//$'\n'/ }"
        return 1
    fi
}

usage() {
    printf 'Usage: %s [--only R1,R12]\n' "${0##*/}" >&2
    exit 2
}

checks=(1 2 4 11 12 13)
if (($#)); then
    [[ $# == 2 && $1 == --only && $2 =~ ^R(1|2|4|11|12|13)(,R(1|2|4|11|12|13))*$ ]] || usage
    requested=,$2,
    selected=()
    for number in "${checks[@]}"; do
        [[ $requested != *",R$number,"* ]] || selected+=("$number")
    done
    checks=("${selected[@]}")
fi
result=0
for number in "${checks[@]}"; do
    run_check "$number" || result=1
done
exit "$result"
