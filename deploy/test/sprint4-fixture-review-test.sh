#!/usr/bin/env bash
set +x
set +a
set -u -o pipefail
umask 077
export LC_ALL=C

# The frozen shim resolves the real Compose model. Every daemon operation and
# application request goes to this shim and acts only on the test's records.
record_call() { printf '%s\n' "$1" >>"$S4_TEST_DIR/calls"; }

object_rows() {
    local kind=$1 filter=$2
    shift 2
    shopt -s nullglob
    local files=("$S4_TEST_DIR/daemon/$kind/"*.json)
    jq -sr "$@" "$filter" "${files[@]}" </dev/null
}

docker_shim() {
    local kind=${1:-} verb=${2:-} project format ref file matched
    local match='select(.Id == $ref or .Name == $ref or (.Id | startswith($ref)))'
    case "$kind $verb" in
        'context show') printf '%s\n' fixture-review ;;
        'context inspect') printf '%s\n' unix:///fixture-review.sock ;;
        'info --format') printf '%s\n' fixture-review-daemon ;;
        'compose '*)
            shift
            if [[ $# == 5 && $1 == --profile && $2 == '*' &&
                $3 == config && $4 == --format && $5 == json ]]; then
                exec "$S4_FROZEN_DOCKER" compose "$@"
            fi
            if [[ $# == 2 && $1 == config && $2 == --quiet ]]; then
                record_call compose-config
                exec "$S4_FROZEN_DOCKER" compose "$@"
            fi
            if [[ $# == 4 && $1 == --profile && ( $2 == setup || $2 == '*' ) &&
                $3 == config && $4 == --quiet ]]; then
                record_call "compose-profile-$2"
                exec "$S4_FROZEN_DOCKER" compose "$@"
            fi
            if [[ $# == 3 && $1 == up && $2 == -d && $3 == server ]]; then
                record_call compose-up
                return 0
            fi
            record_call compose
            return 1
            ;;
        'container ls')
            if [[ $3 == --all ]]; then
                object_rows container '.[] | [.Id, .Name, .Config.Labels["com.docker.compose.project"],
                    .Config.Labels["com.docker.compose.service"]] | join(" ")'
            else
                object_rows container '.[] | select(.State.Running) | [.Id, .Ports] | join(" ")'
            fi
            ;;
        'network ls')
            if [[ $4 == --filter ]]; then
                project=${5#label=com.docker.compose.project=}
                object_rows network '.[] | select(.Labels["com.docker.compose.project"] == $project) | .Id' \
                    --arg project "$project"
            else
                object_rows network '.[] | [.Id, .Name] | join(" ")'
            fi
            ;;
        'volume ls') ;;
        'image ls') object_rows image '.[].Name' ;;
        'inspect --format')
            record_call inspect
            [[ $# == 4 && $3 == '{{.State.Health.Status}}' ]] || return 1
            object_rows container "first(.[] | $match | .State.Health.Status) // empty" -e --arg ref "$4"
            ;;
        'container inspect'|'network inspect'|'image inspect')
            if [[ ${3:-} == --format ]]; then
                format=$4 ref=$5
                [[ $format == '{{.Id}}' ]] || return 1
                object_rows "$kind" "first(.[] | $match | .Id) // empty" -e --arg ref "$ref"
            else
                object_rows "$kind" "[.[] | $match] | select(length == 1)" -e --arg ref "$3"
            fi
            ;;
        'container rm'|'network rm'|'image rm')
            record_call remove
            ref=${@: -1}
            shopt -s nullglob
            for file in "$S4_TEST_DIR/daemon/$kind/"*.json; do
                matched=$(jq -r --arg ref "$ref" "$match | .Id" "$file") || return 1
                if [[ -n $matched ]]; then
                    rm -- "$file"
                    return
                fi
            done
            return 1
            ;;
        *) record_call docker; return 1 ;;
    esac
}

case ${0##*/} in
    docker) docker_shim "$@"; exit $? ;;
    curl)
        record_call curl
        targets=()
        for arg in "$@"; do
            [[ $arg != http://* && $arg != https://* ]] || targets+=("$arg")
        done
        [[ ${#targets[@]} == 1 && ${targets[0]} == "http://127.0.0.1:$LGTM_WEB_PORT/health" ]] || exit 1
        printf '%s\n' 'fixture response'
        exit 0
        ;;
esac

suite_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P) || exit 2
S4_TEST_DOCKER=$(type -P docker)
if [[ -z $S4_TEST_DOCKER ]] || ! "$S4_TEST_DOCKER" compose version >/dev/null 2>&1 ||
    ! command -v jq python3 git >/dev/null; then
    printf '%s\n' 'The fixture review tests require Docker Compose, jq, Python 3 and Git.' >&2
    exit 2
fi

tests=(s4_owned_inspection s4_docker_refusals s4_compose_acceptance s4_compose_refusals
    s4_curl_acceptance s4_curl_refusals s4_foreign_cleanup s4_checkout_divergence)
if [[ $# == 2 && $1 == --only ]]; then
    case "$2" in
        s4_owned_inspection|s4_docker_refusals|s4_compose_acceptance|s4_compose_refusals|\
        s4_curl_acceptance|s4_curl_refusals|s4_foreign_cleanup|s4_checkout_divergence) tests=("$2") ;;
        *) exit 2 ;;
    esac
elif (($#)); then
    exit 2
fi

work=$(mktemp -d) || exit 2
trap 'rm -rf -- "$work"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir "$work/bin" "$work/frozen" &&
    ln -s "$suite_root/deploy/test/sprint4-fixture-review-test.sh" "$work/bin/docker" &&
    ln -s "$suite_root/deploy/test/sprint4-fixture-review-test.sh" "$work/bin/curl" &&
    ln -s "$suite_root/deploy/test/sprint4-fixture-test.sh" "$work/frozen/docker" || exit 2
export S4_TEST_DOCKER S4_FROZEN_DOCKER=$work/frozen/docker PATH="$work/bin:$PATH"

# These are synthetic Docker identifiers. Their prefixes distinguish the objects.
printf -v owned_id 'a%063d' 0
printf -v foreign_id 'b%063d' 0
health_format='{{.State.Health.Status}}'

# A failed assertion prints only its fixed description.
fail() { printf '# %s\n' "$1"; return 1; }
quiet() { "$@" >/dev/null 2>&1; }

in_fixture() (
    local test_name=$1 commit
    S4_TEST_DIR=$(mktemp -d "$work/case.XXXXXXXX") || return 1
    export S4_TEST_DIR TMPDIR="$S4_TEST_DIR/tmp"
    unset DOCKER_HOST DOCKER_CONTEXT
    mkdir -p "$TMPDIR" "$S4_TEST_DIR/daemon/"{container,network,image} || return 1

    quiet source "$suite_root/deploy/test/sprint4-fixture.sh" &&
        commit=$(git -C "$suite_root" rev-parse --verify '89506b7c^{commit}' 2>/dev/null) &&
        quiet sprint4_fixture_open "$suite_root" "$commit" ||
        { fail 'The review case must open a private fixture at the reviewed candidate.'; return 1; }
    if [[ $test_name != s4_checkout_divergence ]]; then
        quiet sprint4_select 4 ||
            { fail 'The review case must select a private candidate stack.'; return 1; }
    fi
    "$test_name"
)

run_label() {
    docker compose --profile '*' config --format json 2>/dev/null |
        jq -er '.services.web.labels["lgtm.sprint4.run"]' 2>/dev/null
}

plant_container() {
    local id=$1 project=$2 service=$3 label=$4 port=${5:-}
    jq -n --arg id "$id" --arg project "$project" --arg service "$service" --arg label "$label" \
        --arg tree "$S4_CANDIDATE_ROOT" --arg port "$port" '
        (if $port == "" then {} else
            {"8080/tcp": [{"HostIp": "127.0.0.1", "HostPort": $port}]} end) as $bindings |
        {Id: $id, Name: ($project + "-" + $service + "-1"),
         Ports: (if $port == "" then "" else "127.0.0.1:" + $port + "->8080/tcp" end),
         Config: {Labels: {
            "lgtm.sprint4.run": $label, "com.docker.compose.project": $project,
            "com.docker.compose.service": $service,
            "com.docker.compose.project.working_dir": ($tree + "/deploy"),
            "com.docker.compose.project.config_files": ($tree + "/deploy/compose.yaml")}},
         Mounts: [], HostConfig: {PortBindings: $bindings},
         State: {Running: true, Health: {Status: "healthy"}}, NetworkSettings: {Ports: $bindings}}
        ' >"$S4_TEST_DIR/daemon/container/$id.json" 2>/dev/null
}

plant_guard_containers() {
    local label
    label=$(run_label) &&
        plant_container "$owned_id" "$COMPOSE_PROJECT_NAME" web "$label" "$LGTM_WEB_PORT" &&
        plant_container "$foreign_id" "other-${COMPOSE_PROJECT_NAME}" server other-owner &&
        quiet sprint4_fixture_ready ||
        fail 'The guard controls must include a ready owned web container and a different container.'
}

driver() {
    # The marker distinguishes a guard refusal from a failed driver preflight.
    sprint4_fixture_run bash -c ': >"$S4_TEST_DIR/driver-entered"; exec "$@"' -- "$@"
}

forwarded() {
    local expected_call=$1 expected_output=$2 output
    shift 2
    : >"$S4_TEST_DIR/calls" || return 1
    output=$(driver "$@" 2>/dev/null) &&
        [[ $output == "$expected_output" && $(<"$S4_TEST_DIR/calls") == "$expected_call" ]]
}

refused() {
    : >"$S4_TEST_DIR/calls" && rm -f -- "$S4_TEST_DIR/driver-entered" || return 1
    if quiet driver "$@"; then return 1; fi
    [[ -f $S4_TEST_DIR/driver-entered && ! -s $S4_TEST_DIR/calls ]]
}

s4_owned_inspection() {
    plant_guard_containers || return 1
    local id status=0
    for id in "$owned_id" "${owned_id:0:12}"; do
        forwarded inspect healthy docker inspect --format "$health_format" "$id" ||
            { fail "The Docker guard must return health for a recorded ${#id}-character container identifier."; status=1; }
    done
    return "$status"
}

s4_docker_refusals() {
    plant_guard_containers || return 1
    local status=0
    refused docker inspect --format "$health_format" "$foreign_id" ||
        { fail 'The Docker guard must refuse the health query for a foreign container.'; status=1; }
    refused docker inspect --format '{{.Id}}' "$owned_id" ||
        { fail 'The Docker guard must refuse a non-health inspection.'; status=1; }
    refused docker container rm -f "$owned_id" ||
        { fail 'The Docker guard must refuse a direct container removal.'; status=1; }
    return "$status"
}

s4_compose_acceptance() {
    local base=$S4_CANDIDATE_ROOT/deploy/compose.yaml status=0 profile
    forwarded compose-config '' docker compose config --quiet ||
        { fail 'The Compose guard must accept validation of the selected configuration.'; status=1; }
    for profile in setup '*'; do
        forwarded "compose-profile-$profile" '' docker compose -f "$base" --profile "$profile" config --quiet ||
            { fail "The Compose guard must accept the selected base and profile $profile."; status=1; }
    done
    forwarded compose-up '' docker compose up -d server ||
        { fail 'The Compose guard must accept starting a service in the selected stack.'; status=1; }
    return "$status"
}

s4_compose_refusals() {
    local status=0
    refused docker compose -p other-project config --quiet ||
        { fail 'The Compose guard must refuse a project supplied with -p.'; status=1; }
    refused docker compose -f "$S4_LEGACY_ROOT/deploy/compose.yaml" config --quiet ||
        { fail 'The Compose guard must refuse a base from the unselected tree.'; status=1; }
    refused docker compose --profile other-profile config --quiet ||
        { fail 'The Compose guard must refuse a profile outside setup and the wildcard.'; status=1; }
    refused docker compose down ||
        { fail 'The Compose guard must refuse the down verb.'; status=1; }
    return "$status"
}

s4_curl_acceptance() {
    plant_guard_containers &&
        forwarded curl 'fixture response' curl --silent --fail "http://127.0.0.1:$LGTM_WEB_PORT/health" ||
        fail 'The curl guard must forward a request to the verified private loopback binding.'
}

s4_curl_refusals() {
    plant_guard_containers || return 1
    local status=0 target="http://127.0.0.1:$LGTM_WEB_PORT/health"
    refused curl "http://localhost:$LGTM_WEB_PORT/health" ||
        { fail 'The curl guard must refuse a host other than the selected numeric loopback.'; status=1; }
    refused curl http://127.0.0.1:8787/health ||
        { fail 'The curl guard must refuse the default stack port.'; status=1; }
    refused curl "$target" "$target" ||
        { fail 'The curl guard must refuse a second request target.'; status=1; }
    refused curl --location "$target" ||
        { fail 'The curl guard must refuse a redirect-following option.'; status=1; }
    # Keep the fixture selected while making the running binding disappear.
    rm -- "$S4_TEST_DIR/daemon/container/$owned_id.json" || return 1
    if quiet sprint4_fixture_ready || ! refused curl "$target"; then
        fail 'The fixture and the curl guard must refuse a request without the owned running binding.'
        status=1
    fi
    return "$status"
}

plant_network_and_image() {
    local owner=$1 network=$2 tag=$3
    jq -n --arg owner "$owner" --arg project "$COMPOSE_PROJECT_NAME" --arg network "$network" '
        {Id: ($network + "-id"), Name: ($project + "_" + $network), Labels: {
            "lgtm.sprint4.run": $owner, "com.docker.compose.project": $project,
            "com.docker.compose.network": $network}}
        ' >"$S4_TEST_DIR/daemon/network/$network.json" 2>/dev/null &&
    jq -n --arg ref "$tag:$LGTM_IMAGE_TAG" --arg tag "$tag" --arg owner "$owner" '
        {Id: ("sha256:" + $tag), Name: $ref, Config: {Labels: {"lgtm.sprint4.run": $owner}}}
        ' >"$S4_TEST_DIR/daemon/image/$tag.json" 2>/dev/null
}

s4_foreign_cleanup() {
    local label kind ref owned_ref status=0
    local fixture=$S4_FIXTURE_DIR tag=$LGTM_IMAGE_TAG
    # The foreign objects match the fixture's names, its project, or its port.
    # Each carries a different ownership label.
    label=$(run_label) &&
        plant_container "$owned_id" "$COMPOSE_PROJECT_NAME" server "$label" &&
        plant_network_and_image "$label" app lgtm-web &&
        plant_container "$foreign_id" "$COMPOSE_PROJECT_NAME" web other-owner "$LGTM_WEB_PORT" &&
        plant_network_and_image other-owner data lgtm-server ||
        { fail 'The owned and foreign cleanup controls must be planted in the shim.'; return 1; }
    local -a kinds=(container network image)
    local -a foreign=("$foreign_id" data-id "lgtm-server:$tag")
    local -a owned=("$owned_id" app-id "lgtm-web:$tag")
    local -a original=()
    local i
    for i in "${!kinds[@]}"; do
        kind=${kinds[i]} ref=${foreign[i]} owned_ref=${owned[i]}
        original[i]=$(docker "$kind" inspect "$ref" 2>/dev/null) &&
            quiet docker "$kind" inspect "$owned_ref" ||
            { fail "The cleanup controls must expose both $kind identities before close."; return 1; }
    done

    quiet sprint4_fixture_close && [[ ! -e $fixture ]] ||
        { fail 'Closing the fixture must succeed and remove its private directory.'; status=1; }
    for i in "${!kinds[@]}"; do
        kind=${kinds[i]} ref=${foreign[i]} owned_ref=${owned[i]}
        if [[ $(docker "$kind" inspect "$ref" 2>/dev/null) != "${original[i]}" ]]; then
            fail "Closing the fixture must preserve the foreign $kind identity."
            status=1
        fi
        if quiet docker "$kind" inspect "$owned_ref"; then
            fail "Closing the fixture must remove the owned $kind."
            status=1
        fi
    done
    return "$status"
}

s4_checkout_divergence() {
    local checkout origin branch base left right
    checkout=$(cd -- "${S4_REPO_B:-/dev/null}" 2>/dev/null && pwd -P) &&
        [[ $checkout == "$S4_FIXTURE_DIR/"* ]] &&
        origin=$(git -C "$checkout" config --get remote.origin.url 2>/dev/null) ||
        { fail 'Repository B must have a private checkout and a fixture origin.'; return 1; }
    origin=${origin#file://}
    [[ $origin == /* ]] || origin=$checkout/$origin
    origin=$(cd -- "$origin" 2>/dev/null && pwd -P) &&
        [[ $origin == "$S4_FIXTURE_DIR/"* && $origin != "$checkout" ]] &&
        branch=$(git -C "$checkout" symbolic-ref --quiet --short HEAD 2>/dev/null) &&
        base=$(git -C "$origin" symbolic-ref --quiet --short HEAD 2>/dev/null) &&
        [[ $branch != "$base" ]] ||
        { fail 'Repository B must be on a branch other than its origin default.'; return 1; }

    # Resolve the heads in their own repositories so the comparison sees either
    # an upstream-only commit or a local commit on the default branch.
    local topic_tip main_tip local_main_tip
    topic_tip=$(git -C "$checkout" rev-parse HEAD 2>/dev/null) &&
        main_tip=$(git -C "$origin" rev-parse "refs/heads/$base" 2>/dev/null) ||
        { fail 'Both repository B branch tips must resolve.'; return 1; }
    local_main_tip=$(git -C "$checkout" rev-parse --verify "refs/heads/$base" 2>/dev/null) || local_main_tip=$main_tip
    local tip
    for tip in "$main_tip" "$local_main_tip"; do
        if GIT_ALTERNATE_OBJECT_DIRECTORIES="$origin/.git/objects" \
            git -C "$checkout" merge-base "$tip" "$topic_tip" >/dev/null 2>&1 &&
            read -r left right < <(GIT_ALTERNATE_OBJECT_DIRECTORIES="$origin/.git/objects" \
                git -C "$checkout" rev-list --left-right --count "$tip...$topic_tip" 2>/dev/null) &&
            ((left > 0 && right > 0)); then
            return 0
        fi
    done
    fail 'Repository B must have at least one exclusive commit on each side of a shared ancestor.'
}

printf '1..%s\n' "${#tests[@]}"
status=0 index=0
for test_name in "${tests[@]}"; do
    index=$((index + 1))
    if in_fixture "$test_name"; then
        printf 'ok %s - %s\n' "$index" "$test_name"
    else
        printf 'not ok %s - %s\n' "$index" "$test_name"
        status=1
    fi
done
exit "$status"
