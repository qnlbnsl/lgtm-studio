#!/usr/bin/env bash
# This suite observes HTTP/2 streams across the Docker-published port of the web server.
set -u -o pipefail
export LC_ALL=C

case "${1:-}" in
    '') rules_only=0 ;;
    --rules-only) rules_only=1 ;;
    *) printf 'Usage: %s [--rules-only]\n' "$0" >&2; exit 2 ;;
esac
if (($# > 1)); then
    printf 'Usage: %s [--rules-only]\n' "$0" >&2
    exit 2
fi

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
caddy=caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0
golang=golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab
distroless=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
work=$(mktemp -d) || exit 1
export BUILDX_CONFIG=$work/buildx
project=lgtm-s2w-${work##*.}
project=${project,,}
network=$project-net
server=$project-server
web=$project-web
compose_started=0
port=''
passed=0
failed=0
skipped=0
open_path=/lgtm.link.v1.LinkService/Open
watch_path=/lgtm.v1.ClientService/WatchClients
join_path=/lgtm.link.v1.LinkService/Join
clients=(basic quiet-up quiet-down default-watch default-join soak-dial soak-up soak-down)

cleanup() {
    local client
    for client in "${clients[@]}"; do
        docker rm -fv "$project-$client" >/dev/null 2>&1 || true
    done
    if ((compose_started)); then
        docker compose -p "$project" down -v --remove-orphans -t 5 >/dev/null 2>&1 || true
        docker image rm "lgtm-web:$project" >/dev/null 2>&1 || true
    elif ((rules_only)); then
        docker rm -fv "$web" "$server" >/dev/null 2>&1 || true
        docker network rm "$network" >/dev/null 2>&1 || true
    fi
    docker rm -fv "$project-build" >/dev/null 2>&1 || true
    rm -rf -- "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

prepare_probe() {
    docker run --rm --name "$project-build" --network none -e CGO_ENABLED=0 \
        -v "$root:/repo:ro" -v "$work:/output" -w /repo "$golang" \
        go build -trimpath -buildvcs=false -o /output/probe ./deploy/test/streamprobe || return 1

    if ! docker image inspect "$distroless" >/dev/null 2>&1; then
        # Use anonymous registry access when the host's credential helper is configured.
        mkdir "$work/registry" || return 1
        printf '%s\n' '{"auths":{"gcr.io":{}}}' >"$work/registry/config.json"
        local engine
        engine=$(docker info --format '{{.ID}}') || return 1
        [[ $(docker --config "$work/registry" info --format '{{.ID}}') == "$engine" ]] || return 1
        docker --config "$work/registry" pull "$distroless" || return 1
    fi
}

setup_rules_only() {
    docker network create "$network" >/dev/null &&
        docker run -d --name "$server" --network "$network" --network-alias server \
            --read-only --cap-drop ALL --security-opt no-new-privileges:true \
            -v "$work/probe:/probe:ro" --entrypoint /probe "$distroless" -mode serve -addr :8080 >/dev/null &&
        docker run -d --name "$web" --network "$network" --publish 127.0.0.1::8080 \
            --user 10003:10003 --read-only --cap-drop ALL --security-opt no-new-privileges:true \
            --tmpfs /tmp -e XDG_DATA_HOME=/tmp -e XDG_CONFIG_HOME=/tmp \
            -v "$root/deploy/Caddyfile:/etc/caddy/Caddyfile:ro" "$caddy" >/dev/null
}

setup_compose() {
    local override=$work/stream.compose.yaml
    local library=$root/deploy/test/web-testlib.sh
    [[ -r $library ]] || { printf 'Item 4 must supply deploy/test/web-testlib.sh for this run.\n' >&2; return 1; }
    # Compose quotes this bind source as a string even when the temporary path contains spaces.
    cat >"$override" <<'OVERRIDE'
services:
  server:
    build: !reset null
    image: ${LGTM_STREAM_PROBE_IMAGE:?}
    entrypoint: ["/probe"]
    command: ["-mode", "serve", "-addr", ":8080"]
    depends_on: !reset []
    environment: !reset {}
    healthcheck: !reset null
    volumes: !override
      - type: bind
        source: ${LGTM_STREAM_PROBE_BINARY:?}
        target: /probe
        read_only: true
        bind:
          create_host_path: false
  web:
    depends_on: !reset []
    ports: !override
      - host_ip: 127.0.0.1
        published: "0"
        target: 8080
        protocol: tcp
OVERRIDE

    # Read the fixture's paths in a subshell so its helpers cannot replace this suite's functions.
    (
        cd "$root" || exit 1
        export TMPDIR=$work
        source "$library" && web_fixture_create && web_fixture_override "$override" || exit 1
        printf '%s\0' "$COMPOSE_FILE" "$COMPOSE_ENV_FILES" "$LGTM_STACK_STATE_DIR" >&3
    ) 3>"$work/fixture-paths" || return 1
    local -a paths=()
    mapfile -d '' -t paths <"$work/fixture-paths"
    [[ ${#paths[@]} == 3 ]] || return 1
    local folder
    folder=$(dirname -- "${paths[1]}") || return 1
    [[ $folder == "$work"/tmp.* && $(dirname -- "$folder") == "$work" &&
        ${paths[2]} == "$folder/state" ]] || return 1

    unset LGTM_WEB_PORT LGTM_WEB_PASSWORD COMPOSE_PROFILES
    export COMPOSE_FILE=${paths[0]} COMPOSE_ENV_FILES=${paths[1]} LGTM_STACK_STATE_DIR=${paths[2]}
    export COMPOSE_PROJECT_NAME=$project LGTM_IMAGE_TAG=$project
    export LGTM_STREAM_PROBE_IMAGE=$distroless LGTM_STREAM_PROBE_BINARY=$work/probe
    local image="lgtm-web:$project" built_image config
    config=$(docker compose -p "$project" config --format json) || return 1
    jq -e --arg project "$project" --arg image "$image" --arg probe "$work/probe" --arg base "$distroless" '
        .name == $project
        and ((.networks // {}) + (.volumes // {}) | all(.[]; (.name | startswith($project)) and (.external != true)))
        and (.services.server |
            .image == $base
            and (.container_name == null or (.container_name | startswith($project)))
            and .build == null and .healthcheck == null
            and ((.depends_on // {}) | length == 0)
            and ((.environment // {}) | length == 0)
            and ((.ports // []) | length == 0)
            and .entrypoint == ["/probe"]
            and ((.volumes // []) | length == 1)
            and (.volumes[0] | .type == "bind" and .source == $probe and .target == "/probe" and .read_only))
        and (.services.web |
            .image == $image
            and (.container_name == null or (.container_name | startswith($project)))
            and ((.depends_on // {}) | length == 0)
            and ((.volumes // []) | length == 0)
            and ((.ports // []) | length == 1)
            and (.ports[0] |
                .host_ip == "127.0.0.1" and ((.published // 0) | tostring) == "0" and .target == 8080))
    ' >/dev/null <<<"$config" || { printf 'The Compose override did not resolve to the isolated probe fixture.\n' >&2; return 1; }

    compose_started=1
    docker compose -p "$project" build web &&
        built_image=$(docker image inspect --format '{{.Id}}' "$image") &&
        docker compose -p "$project" up -d --no-deps server web &&
        web=$(docker compose -p "$project" ps -q web) &&
        server=$(docker compose -p "$project" ps -a -q server) &&
        [[ $(docker inspect --format '{{.Image}}' "$web") == "$built_image" ]] &&
        docker compose -p "$project" exec -T web cat /etc/caddy/Caddyfile | cmp -s "$root/deploy/Caddyfile" -
}

setup() {
    prepare_probe || return 1
    if ((rules_only)); then
        setup_rules_only || return 1
    else
        setup_compose || return 1
    fi
    local binding deadline=$((SECONDS + 15))
    binding=$(docker port "$web" 8080/tcp) || return 1
    [[ $binding == 127.0.0.1:* ]] || return 1
    port=${binding##*:}
    until curl --noproxy '*' --silent --max-time 2 --output /dev/null "http://127.0.0.1:$port/"; do
        ((SECONDS < deadline)) || return 1
        sleep 0.2
    done
}

probe() {
    local client=$1 seconds=$2 path=$3
    shift 3
    timeout --kill-after=5s "$((seconds + 60))" docker run --rm --name "$project-$client" --network host \
        --read-only --cap-drop ALL --security-opt no-new-privileges:true \
        -v "$work/probe:/probe:ro" --entrypoint /probe "$distroless" \
        "$@" -url "http://127.0.0.1:$port$path"
}

open_probe() {
    local client=$1 seconds=$2 start=$SECONDS
    shift 2
    probe "$client" "$seconds" "$open_path" "$@" || return 1
    if ((SECONDS - start + 2 < seconds)); then
        printf 'The probe ended after %ds. The expected duration was about %ds.\n' "$((SECONDS - start))" "$seconds"
        return 1
    fi
}

stream_passes_both_ways_loopback() {
    local log=$work/basic.log
    open_probe basic 55 -mode dial -every 2s -total 20s -gap 35s >"$log" 2>&1 || return 1
    grep -Eq '^ok stream lasted .+ with 11 messages each way([[:space:]]|$)' "$log" &&
        grep -Eiq '\btrailer[=:[:space:]]+11([[:space:]]|$)' "$log"
}

stream_hops_speak_http2() {
    grep -Fxq 'proto HTTP/2.0' "$work/basic.log" &&
        docker logs "$server" >"$work/upstream.log" 2>&1 &&
        grep -Fq "upstream proto=HTTP/2.0 path=$open_path" "$work/upstream.log"
}

check_quiet_up() {
    local log=$1 minimum=$2
    grep -Eq '^ok([[:space:]]|$)' "$log" &&
        grep -Eq '\back m1\b' "$log" && grep -Eiq '\btrailer[=:[:space:]]+up-closed([[:space:]]|$)' "$log" &&
        awk -v minimum="$minimum" '
            match($0, /[0-9]+ ticks/) && substr($0, RSTART, RLENGTH) + 0 >= minimum { found = 1 }
            END { exit !found }
        ' "$log"
}

check_quiet_down() {
    local log=$1
    grep -Eq '^ok([[:space:]]|$)' "$log" &&
        grep -Eq '\bcount [1-9][0-9]*\b' "$log" &&
        grep -Eiq '\btrailer[=:[:space:]]+down-closed([[:space:]]|$)' "$log"
}

stream_survives_upward_silence() {
    local log=$work/quiet-up.log
    open_probe quiet-up 80 -mode shaped -shape quiet-up -total 80s >"$log" 2>&1 &&
        check_quiet_up "$log" 38
}

stream_survives_downward_silence() {
    local log=$work/quiet-down.log
    open_probe quiet-down 80 -mode shaped -shape quiet-down -total 80s >"$log" 2>&1 &&
        check_quiet_down "$log"
}

# Record the duration of the failed probe for each route, independently of the other jobs.
default_cut() {
    local client=$1 path=$2 log=$work/$1.log
    local start=$SECONDS result=0 elapsed
    probe "$client" 80 "$path" -mode shaped -shape quiet-up -total 80s >"$log" 2>&1 || result=$?
    elapsed=$((SECONDS - start))
    printf 'The probe exited %d after %ds. %s\n' "$result" "$elapsed" "$path" >>"$log"
    [[ $result == 1 && $elapsed -ge 55 && $elapsed -le 70 ]] &&
        grep -Eq '^not ok([[:space:]]|$)' "$log" &&
        docker logs "$server" 2>&1 | awk -v path="$path" '
            index($0, "upstream proto=HTTP/2.0 path=" path) { found = 1 }
            END { exit !found }
        '
}

other_routes_keep_the_default_limit() {
    default_cut default-watch "$watch_path" &
    local watch=$!
    default_cut default-join "$join_path" &
    local join=$! result=0
    wait "$watch" || result=1
    wait "$join" || result=1
    return "$result"
}

server_stream_arrives_message_by_message() {
    local log=$work/ticks.log
    curl --noproxy '*' --http1.1 --no-buffer --silent --show-error --fail --max-time 140 \
        --data '' --header 'Probe-Shape: ticks' --header 'Probe-Ticks: 40' \
        --dump-header "$work/tick-headers" "http://127.0.0.1:$port$watch_path" |
        while IFS= read -r line; do
            printf '%s %s\n' "${EPOCHREALTIME/./}" "$line"
        done >"$log" &&
        grep -Eq '^HTTP/1\.1 200([[:space:]]|$)' "$work/tick-headers" &&
        awk '
            NF != 3 || $2 != "tick" || $3 !~ /^[0-9]+$/ { bad = 1 }
            NR > 1 && ($1 - arrived < 1500000 || $3 != previous + 1) { bad = 1 }
            { arrived = $1; previous = $3 }
            END { exit !(NR == 40 && !bad) }
        ' "$log"
}

stream_survives_soak() {
    open_probe soak-dial 364 -mode dial -every 2s -total 4s -gap 6m >"$work/soak-dial.log" 2>&1 &
    local dial=$!
    open_probe soak-up 390 -mode shaped -shape quiet-up -total 6m30s >"$work/soak-up.log" 2>&1 &
    local up=$!
    open_probe soak-down 390 -mode shaped -shape quiet-down -total 6m30s >"$work/soak-down.log" 2>&1 &
    local down=$! result=0
    wait "$dial" || result=1
    wait "$up" || result=1
    wait "$down" || result=1
    ((result == 0)) &&
        grep -Eq '^ok stream lasted .+ with 3 messages each way([[:space:]]|$)' "$work/soak-dial.log" &&
        grep -Eiq '\btrailer[=:[:space:]]+3([[:space:]]|$)' "$work/soak-dial.log" &&
        check_quiet_up "$work/soak-up.log" 193 && check_quiet_down "$work/soak-down.log"
}

record() {
    local name=$1 status=$2
    shift 2
    if ((status == 0)); then
        printf 'PASS [%s]\n' "$name"
        ((passed += 1))
    else
        printf 'FAIL [%s]\n' "$name"
        ((failed += 1))
        local log
        for log; do
            if [[ -s $work/$log.log ]]; then
                printf '  %s\n' "$log"
                tail -n 8 "$work/$log.log" | sed 's/^/    /'
            fi
        done
    fi
}

record_job() {
    local name=$1 pid=$2 status=0
    shift 2
    wait "$pid" || status=$?
    record "$name" "$status" "$@"
}

if ! setup >"$work/setup.log" 2>&1; then
    record setup 1 setup
    printf '0 passed, 1 failed, 0 skipped\n'
    exit 1
fi

stream_passes_both_ways_loopback & basic=$!
stream_survives_upward_silence & up=$!
stream_survives_downward_silence & down=$!
other_routes_keep_the_default_limit & default=$!
server_stream_arrives_message_by_message & ticks=$!
record_job stream_passes_both_ways_loopback "$basic" basic
status=0
stream_hops_speak_http2 || status=$?
record stream_hops_speak_http2 "$status" basic upstream
record_job stream_survives_upward_silence "$up" quiet-up
record_job stream_survives_downward_silence "$down" quiet-down
record_job other_routes_keep_the_default_limit "$default" default-watch default-join
record_job server_stream_arrives_message_by_message "$ticks" ticks
if [[ ${LGTM_STREAM_SOAK:-} == 1 ]]; then
    status=0
    stream_survives_soak || status=$?
    record stream_survives_soak "$status" soak-dial soak-up soak-down
else
    printf 'PASS [stream_survives_soak] # SKIP soak not requested\n'
    skipped=1
fi
printf '%d passed, %d failed, %d skipped\n' "$passed" "$failed" "$skipped"
((failed == 0))
