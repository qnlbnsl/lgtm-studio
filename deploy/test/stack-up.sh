#!/usr/bin/env bash
# This suite observes the launcher through real and stand-in Compose services.
set -u -o pipefail
export LC_ALL=C

source "$(dirname -- "${BASH_SOURCE[0]}")/web-testlib.sh"
web_passed=0 web_failed=0
vault_launcher_pid=''
vault_phases=(database seal vault ready)
full_phases=(database seal vault images server web ready)
service_path=/lgtm.v1.SessionService
settings_error='the settings cannot be read; check the quoting in deploy/.env'
password_error='LGTM_WEB_PASSWORD is missing or invalid.'
trap 'exit 130' INT
trap 'exit 143' TERM

dc() { docker compose "$@"; }

settings() {
    printf 'LGTM_WEB_PORT=%s\n' "$web_port" >"$COMPOSE_ENV_FILES" || return 1
    if (($#)); then
        printf 'LGTM_WEB_PASSWORD=%s\n' "$1" >>"$COMPOSE_ENV_FILES"
    fi
}

setup() {
    web_fixture_create || return 1
    export BUILDX_CONFIG=$web_folder/buildx
    trap 'result=$?; cleanup || result=1; exit "$result"' EXIT
    launcher=$web_root/deploy/up.sh
    launch_dir=$web_root
    output=$web_folder/up.log
    times=$web_folder/phases.log
}

cleanup() {
    if [[ -n ${vault_launcher_pid:-} ]]; then
        kill "$vault_launcher_pid" 2>/dev/null || true
        wait "$vault_launcher_pid" 2>/dev/null || true
    fi
    if [[ -n ${blocker:-} ]]; then
        docker rm -fv "$blocker" >/dev/null 2>&1 || true
    fi
    if [[ -n ${other_project:-} ]]; then
        web_fixture_remove_project "$other_project" || return 1
    fi
    web_fixture_cleanup
}

# Capture both streams and record when the launcher prints each phase line.
launch() {
    local -a statuses
    (cd -- "$launch_dir" && timeout 1000 "$launcher" "$@") 2>&1 |
        while IFS= read -r line || [[ -n $line ]]; do
            printf '%s\n' "$line"
            if [[ $line == 'phase '* ]]; then
                printf '%s %s\n' "$EPOCHREALTIME" "$line" >>"$times"
            fi
        done >"$output"
    statuses=("${PIPESTATUS[@]}")
    rc=${statuses[0]}
    ((statuses[1] == 0))
}

run_up() {
    : >"$times"
    launch "$@"
}

phase_lines() { grep '^phase ' "$output"; }
last_line() { awk 'NF { line = $0 } END { print line }' "$output"; }

expected_phases() {
    local phase
    for phase in "$@"; do
        printf 'phase %s start\nphase %s done\n' "$phase" "$phase"
    done
}

completed_phases() { [[ $(phase_lines) == "$(expected_phases "$@")" ]]; }

failed_in_phase() {
    local failed=$1
    shift
    [[ $(phase_lines) == "$(expected_phases "$@"; printf 'phase %s start\n' "$failed")" ]]
}

cid() {
    local id
    id=$(dc ps -a -q "$1") && [[ -n $id && $id != *$'\n'* ]] || return 1
    printf '%s\n' "$id"
}

inspect_service() {
    local id
    id=$(cid "$1") || return 1
    docker inspect --format "$2" "$id"
}

has_no_container() {
    local ids
    ids=$(docker ps -aq --filter "label=com.docker.compose.project=$web_project" \
        --filter "label=com.docker.compose.service=$1") || return 1
    [[ -z $ids ]]
}

healthy() { [[ $(inspect_service "$1" '{{.State.Running}} {{.State.Health.Status}}') == 'true healthy' ]]; }

# Reject a model that would create resources outside the fixture or publish another port.
safe_model() {
    local model status=0
    model=$(dc --profile '*' config --format json) || return 1
    web_fixture_owns_model <<<"$model" &&
        jq -e --arg port "$web_port" --arg tag "$web_project" --arg caddy "$web_caddy" '
            .services as $s
            | ($s.web.ports | length) == 1 and ($s.web.ports[0] |
                .host_ip == "127.0.0.1" and .published == $port and .target == 8080)
            and ($s | del(.web) | all(.[]; (.ports // []) == []))
            and $s.web.image == ("lgtm-web:" + $tag)
            and ($s.server.image == ("lgtm-server:" + $tag) or $s.server.image == $caddy)
        ' <<<"$model" >/dev/null || status=$?
    if ((status)); then
        printf '  The Compose model does not assign the web port and images to the fixture.\n' >&2
    fi
    return "$status"
}

needs_real_server() {
    [[ -f $web_root/src/server/cmd/lgtm-server/main.go ]] || {
        printf '  This check requires the real server entry point from Server item 9.\n' >&2
        return 1
    }
}

elapsed() {
    awk -v phase="$1" -v end="$2" -v minimum="$3" -v maximum="$4" '
        $2 == "phase" && $3 == phase && $4 == "start" { start = $1 }
        $2 == "phase" && $3 == phase && $4 == "done" { end = $1 }
        END { exit !(start && end && end - start > minimum && end - start < maximum) }
    ' "$times"
}

stand_in_server() {
    local script=$1 override=$web_folder/server.yaml
    printf '%s\n' "$script" >"$web_folder/server-command"
    cat >"$override" <<EOF_SERVER
services:
  server:
    build: !reset null
    image: $web_caddy
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |-
        $script
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/healthz"]
EOF_SERVER
    web_fixture_override "$override" && safe_model || return 1
    dc config --format json |
        jq -e --arg image "$web_caddy" --rawfile script "$web_folder/server-command" '
            .services.server | .image == $image and .entrypoint == ["/bin/sh", "-c"]
            and .command == [($script | rtrimstr("\n"))] and .build == null
            and .restart == "unless-stopped"
            and .healthcheck == {
                test:["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/healthz"],
                start_period:"8m0s", start_interval:"2s", interval:"10s", timeout:"3s", retries:3}
        ' >/dev/null
}

# Block image builds and application starts in checks that only exercise the vault route.
restrict_to_vault() {
    local docker_path
    docker_path=$(command -v docker) || return 1
    export WEB_TEST_DOCKER=$docker_path WEB_TEST_COMPOSE_CALLS=$web_folder/compose-calls
    mkdir "$web_folder/bin" || return 1
    cat >"$web_folder/bin/docker" <<'EOF_DOCKER'
#!/usr/bin/env bash
if [[ $1 == compose ]]; then
    printf '%s\n' "$*" >>"$WEB_TEST_COMPOSE_CALLS"
    for arg in "$@"; do
        case $arg in
            build|server|web) printf 'Unexpected application operation on the vault route.\n' >&2; exit 77 ;;
        esac
    done
fi
exec "$WEB_TEST_DOCKER" "$@"
EOF_DOCKER
    chmod +x "$web_folder/bin/docker" || return 1
    export PATH=$web_folder/bin:$PATH
}

up_until_vault_needs_no_web_settings() {
    setup && restrict_to_vault || return 1
    LGTM_WEB_PORT=8787 dc --profile '*' config --format json | web_fixture_owns_model || return 1
    printf 'LGTM_WEB_PORT=bad\n' >"$COMPOSE_ENV_FILES"
    run_up --until vault && ((rc == 0)) && completed_phases "${vault_phases[@]}" &&
        [[ $(last_line) == 'postgres and openbao are healthy' ]] || return 1
    : >"$COMPOSE_ENV_FILES"
    LGTM_WEB_PORT=bad run_up --until vault && ((rc == 0)) && completed_phases "${vault_phases[@]}" &&
        [[ $(last_line) == 'postgres and openbao are healthy' ]] &&
        has_no_container server && has_no_container web &&
        ! docker image inspect "lgtm-server:$web_project" >/dev/null 2>&1 &&
        ! docker image inspect "lgtm-web:$web_project" >/dev/null 2>&1
}

rpc() {
    local method=$1 input=$2
    shift 2
    http_status=$(curl --silent --show-error --noproxy '*' --max-time 10 \
        --header 'Content-Type: application/json' --header 'Connect-Protocol-Version: 1' \
        --data-binary @- --dump-header "$web_folder/headers" --output "$web_folder/body" \
        --write-out '%{http_code}' "$url$service_path/$method" "$@" <<<"$input")
}

sign_in() {
    local origin=$1 json
    json=$(WEB_TEST_PASSWORD=$web_password jq -nc '{password: env.WEB_TEST_PASSWORD}') || return 1
    rpc SignIn "$json" --header "Origin: $origin" --cookie-jar "$web_folder/session.cookies" &&
        [[ $http_status == 200 ]] && grep -iq '^set-cookie:' "$web_folder/headers" &&
        jq -e '.result == "SIGN_IN_RESULT_SIGNED_IN" or .result == 1' "$web_folder/body" >/dev/null
}

signed_in() {
    rpc GetServer '{}' --cookie "$web_folder/session.cookies" &&
        [[ $http_status == 200 ]] && jq -e '.signedIn == true' "$web_folder/body" >/dev/null
}

website_line() { [[ $(last_line) == "the website is at http://127.0.0.1:$web_port/" ]]; }

# Reproduce the operator's lookup from a checkout with competing root and deployment settings.
file_only_project_and_port() (
    local checkout=$web_folder/checkout
    mkdir "$checkout" || exit 1
    (
        cd "$web_root" || exit 1
        git ls-files -z -- . ':(exclude).env' ':(exclude)deploy/.env' ':(exclude)deploy/.state/**' |
            tar -c --ignore-failed-read --no-recursion --null -T - |
            tar -xf - -C "$checkout"
    ) || exit 1
    cp -- "$web_root"/deploy/{server.Dockerfile,server.Dockerfile.dockerignore,website.Dockerfile,website.Dockerfile.dockerignore,compose.yaml,up.sh,Caddyfile} \
        "$checkout/deploy/" || exit 1
    # Generate a different project and port for the decoy settings file.
    (umask 077; printf 'LGTM_WEB_PORT=%s\nCOMPOSE_PROJECT_NAME=%s\n' "$other_port" "$other_project" >"$checkout/.env" &&
        printf 'LGTM_WEB_PORT=%s\nCOMPOSE_PROJECT_NAME=%s\nLGTM_WEB_PASSWORD=%s\n' \
            "$web_port" "$web_project" "$web_password" >"$checkout/deploy/.env") || exit 1
    unset COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PROJECT_NAME LGTM_WEB_PORT LGTM_WEB_PASSWORD
    launcher=$checkout/deploy/up.sh launch_dir=$checkout
    cd -- "$checkout" || exit 1
    local model
    model=$(docker compose -f deploy/compose.yaml config --format json) || exit 1
    web_fixture_owns_model <<<"$model" || exit 1
    run_up && ((rc == 0)) && website_line &&
        [[ $(docker compose -f deploy/compose.yaml port web 8080) == "127.0.0.1:$web_port" ]]
)

up_starts_everything_in_order() {
    needs_real_server && setup && settings && safe_model || return 1
    url=http://127.0.0.1:$web_port
    LGTM_WEB_PASSWORD=$web_password run_up && ((rc == 0)) &&
        completed_phases "${full_phases[@]}" && website_line &&
        healthy postgres && healthy openbao && healthy server && healthy web &&
        curl --noproxy '*' -fsS --max-time 10 "$url/" >"$web_folder/index.html" &&
        grep -Fq 'id="app"' "$web_folder/index.html" &&
        sign_in "$url" && sign_in "http://localhost:$web_port" || return 1

    # Run the file-only case on a different disposable project and empty state.
    web_fixture_cleanup || return 1
    setup || return 1
    other_project=lgtm-s2w-$(web_fixture_random 4)
    other_port=$((web_port == 39999 ? 39998 : web_port + 1))
    file_only_project_and_port
}

up_starts_no_server_before_the_vault() {
    setup && safe_model || return 1
    run_up --until vault && ((rc == 0)) || return 1
    docker run --rm --name "$web_project-malform" --user 0:0 --network none \
        --volume "$web_folder:/fixture" --entrypoint sh "$web_caddy" \
        -ec 'printf "{malformed" >/fixture/state/server/openbao.json' || return 1
    run_up && ((rc > 0 && rc < 124)) && failed_in_phase vault database seal &&
        grep -Fq 'server vault credential needs repair' "$output" &&
        has_no_container server && has_no_container web
}

up_waits_for_a_slow_server() {
    setup &&
        stand_in_server 'echo slow-stand-in; sleep 150; exec caddy respond --listen :8080 ok' &&
        run_up && ((rc == 0)) && completed_phases "${full_phases[@]}" &&
        elapsed server 0 150 540 && healthy server &&
        dc logs --no-log-prefix server | grep 'slow-stand-in' >/dev/null
}

up_stops_on_a_failed_server() {
    setup &&
        stand_in_server 'echo failed-stand-in >&2; echo "LGTM_WEB_PASSWORD is missing or invalid." >&2; exit 1' &&
        run_up && ((rc == 1)) && failed_in_phase server database seal vault images &&
        elapsed server "$EPOCHREALTIME" 0 60 && grep -Fxq failed-stand-in "$output" &&
        grep -Fxq "$password_error" "$output" || return 1
    local before after
    before=$(inspect_service server '{{.Id}} {{.State.Status}} {{.RestartCount}}') || return 1
    [[ $before == *' exited '* ]] || return 1
    sleep 20
    after=$(inspect_service server '{{.Id}} {{.State.Status}} {{.RestartCount}}') || return 1
    [[ $before == "$after" ]] && has_no_container web
}

up_names_a_taken_port() {
    setup || return 1
    blocker=$web_project-blocker
    # A host socket produces the bind error. Two Docker publications collide in its port allocator.
    docker run -d --name "$blocker" --network host --user 10003:10003 --read-only --cap-drop ALL \
        --security-opt no-new-privileges:true \
        --entrypoint caddy "$web_caddy" respond --listen "127.0.0.1:$web_port" occupied >/dev/null || return 1
    local deadline=$((SECONDS + 10))
    until [[ $(curl --noproxy '*' -fsS --max-time 2 "http://127.0.0.1:$web_port/" 2>/dev/null) == occupied ]]; do
        ((SECONDS < deadline)) || return 1
        sleep 1
    done
    stand_in_server 'exec caddy respond --listen :8080 ok' &&
        run_up && ((rc == 1)) && failed_in_phase web database seal vault images server &&
        grep -Fq 'address already in use' "$output" &&
        healthy postgres && healthy openbao && healthy server || return 1
    docker rm -f "$blocker" >/dev/null || return 1
    blocker=''
    run_up && ((rc == 0)) && website_line && healthy web &&
        [[ $(dc port web 8080) == "127.0.0.1:$web_port" ]] &&
        curl --noproxy '*' -sS --max-time 5 --output /dev/null "http://127.0.0.1:$web_port/"
}

up_is_idempotent() {
    needs_real_server && setup && safe_model && run_up && ((rc == 0)) || return 1
    local server_before web_before
    server_before=$(inspect_service server '{{.Id}} {{.Image}}') &&
        web_before=$(inspect_service web '{{.Id}} {{.Image}}') &&
        run_up && ((rc == 0)) && website_line &&
        [[ $(inspect_service server '{{.Id}} {{.Image}}') == "$server_before" ]] &&
        [[ $(inspect_service web '{{.Id}} {{.Image}}') == "$web_before" ]]
}

revocation_commits_and_stops_server() {
    run_up && ((rc == 1)) && failed_in_phase server database seal vault images &&
        grep -Fxq 'Session revocation committed.' "$output" && grep -Fxq "$password_error" "$output" &&
        [[ $(inspect_service server '{{.State.Status}}') == exited ]]
}

old_cookie_is_refused() {
    rpc GetServer '{}' --cookie "$web_folder/session.cookies" && [[ $http_status == 200 ]] &&
        jq -e '(.signedIn // false) == false' "$web_folder/body" >/dev/null &&
        rpc SignOut '{}' --cookie "$web_folder/session.cookies" && [[ $http_status == 401 ]] &&
        jq -e '.code == "unauthenticated"' "$web_folder/body" >/dev/null
}

up_password_removal_ends_sessions() {
    needs_real_server && setup && safe_model && run_up && ((rc == 0)) || return 1
    url=http://127.0.0.1:$web_port
    sign_in "$url" && signed_in && settings || return 1

    # A password still exported in the shell takes precedence over the file.
    LGTM_WEB_PASSWORD=$web_password run_up && ((rc == 0)) && signed_in || return 1
    local before after
    before=$(cid server) || return 1
    unset LGTM_WEB_PASSWORD
    revocation_commits_and_stops_server && after=$(cid server) && [[ $before != "$after" ]] &&
        settings "$web_password" &&
        run_up && ((rc == 0)) && old_cookie_is_refused || return 1

    # The launcher restarts PostgreSQL, so make it unreachable from the server only.
    sign_in "$url" && signed_in && dc stop postgres >/dev/null && settings || return 1
    local original_files=$COMPOSE_FILE
    cat >"$web_folder/database-unavailable.yaml" <<'EOF_DATABASE'
services:
  server:
    environment:
      LGTM_POSTGRES_HOST: unavailable-postgres.invalid
EOF_DATABASE
    web_fixture_override "$web_folder/database-unavailable.yaml" && safe_model &&
        run_up && ((rc == 1)) && failed_in_phase server database seal vault images &&
        grep -Fxq 'Session revocation was not confirmed.' "$output" &&
        ! grep -Fxq 'Session revocation committed.' "$output" &&
        [[ $(inspect_service server '{{.State.Status}}') == exited ]] || return 1
    export COMPOSE_FILE=$original_files
    revocation_commits_and_stops_server && settings "$web_password" &&
        run_up && ((rc == 0)) && old_cookie_is_refused
}

up_keeps_the_password_out_of_its_output() {
    setup && restrict_to_vault || return 1
    local marker option
    marker=$(web_fixture_random 16) || return 1
    printf 'LGTM_WEB_PASSWORD="%s\n' "$marker" >"$COMPOSE_ENV_FILES"
    for option in full vault; do
        local -a args=()
        if [[ $option == vault ]]; then args=(--until vault); fi
        : >"$WEB_TEST_COMPOSE_CALLS"
        run_up "${args[@]}" && ((rc == 2)) && grep -Fxq "$settings_error" "$output" &&
            ! grep -q '^phase ' "$output" && [[ $(<"$output") != *"$marker"* ]] &&
            [[ $(wc -l <"$WEB_TEST_COMPOSE_CALLS") -eq 1 ]] &&
            grep -Eq '^compose( .+)? config (--quiet|-q)$' "$WEB_TEST_COMPOSE_CALLS" || return 1
    done
    : >"$WEB_TEST_COMPOSE_CALLS"
    run_up --remove-password && ((rc == 2)) &&
        grep -Fxq 'usage: deploy/up.sh [--until vault]' "$output" &&
        ! grep -q '^phase ' "$output" && [[ $(<"$output") != *"$marker"* && ! -s $WEB_TEST_COMPOSE_CALLS ]]
}

up_keeps_changed_settings_password_out_of_its_output() {
    setup && safe_model && restrict_to_vault || return 1
    local lock_fd marker=synthetic-lock-wait-test-marker status=0 deadline
    mkdir -m 0700 -- "$LGTM_STACK_STATE_DIR" || return 1
    exec {lock_fd}<"$LGTM_STACK_STATE_DIR" || return 1
    flock --exclusive "$lock_fd" || return 1

    # Close the fixture's lock descriptor in the launcher so it holds only its own lock.
    (cd -- "$launch_dir" && exec timeout --kill-after=5 180 "$launcher" --until vault) \
        {lock_fd}<&- >"$output" 2>&1 &
    vault_launcher_pid=$!
    deadline=$((SECONDS + 30))
    # The timeout process owns the launcher's process group. The kernel reports a blocked flock.
    until ps -e -o pgid=,comm=,wchan= |
        awk -v group="$vault_launcher_pid" '
            $1 == group && $2 == "flock" && $3 ~ /lock.*wait/ { waiting = 1 }
            END { exit !waiting }
        '; do
        if ((SECONDS >= deadline)) || ! kill -0 "$vault_launcher_pid" 2>/dev/null; then
            printf '  The launcher did not wait for the fixture state lock.\n' >&2
            return 1
        fi
        sleep 0.05
    done

    printf 'LGTM_WEB_PORT=%s\nLGTM_WEB_PASSWORD="%s\n' "$web_port" "$marker" >"$COMPOSE_ENV_FILES" || return 1
    flock --unlock "$lock_fd" || return 1
    exec {lock_fd}<&-
    wait "$vault_launcher_pid" || status=$?
    vault_launcher_pid=''
    if ((status >= 124)); then
        printf '  The launcher did not finish normally after the fixture released the lock.\n' >&2
        return 1
    fi
    if [[ $(<"$output") == *"$marker"* ]]; then
        printf '  The launcher exposed the password from settings changed during the lock wait.\n' >&2
        return 1
    fi
}

assertions=(up_until_vault_needs_no_web_settings up_starts_everything_in_order
    up_starts_no_server_before_the_vault up_waits_for_a_slow_server up_stops_on_a_failed_server
    up_names_a_taken_port up_is_idempotent up_password_removal_ends_sessions up_keeps_the_password_out_of_its_output
    up_keeps_changed_settings_password_out_of_its_output)

if [[ $* == --stand-ins ]]; then
    assertions=(up_until_vault_needs_no_web_settings up_starts_no_server_before_the_vault
        up_waits_for_a_slow_server up_stops_on_a_failed_server up_names_a_taken_port
        up_keeps_the_password_out_of_its_output up_keeps_changed_settings_password_out_of_its_output)
elif (($#)); then
    printf 'Usage: %s [--stand-ins]\n' "$0" >&2
    exit 2
fi

for assertion in "${assertions[@]}"; do
    web_check "$assertion"
done
web_finish
