#!/usr/bin/env bash
set -u
set -o pipefail
export LC_ALL=C

cd -- "${BASH_SOURCE[0]%/*}/../.." || exit 2

compose_file=deploy/compose.yaml
up_file=deploy/up.sh
client_package=./src/client/cmd/lgtm-client
mutation_hook=deploy/test/mutate-feed-lock.sh

fail() {
    printf '%s\n' "$*"
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is missing."
}

compose() {
    docker compose -f "$compose_file" "$@"
}

require_stack() {
    [[ -f $compose_file ]] || fail "$compose_file is missing."
    need docker
    docker compose version >/dev/null 2>&1 || fail "Docker Compose is unavailable."
}

configure_web() {
    local port=${LGTM_WEB_PORT:-8787}
    [[ $port =~ ^[0-9]{1,5}$ ]] && (( 10#$port >= 1 && 10#$port <= 65535 )) ||
        fail "LGTM_WEB_PORT is outside the valid port range."
    web_url="http://127.0.0.1:$((10#$port))"
    need curl
    need jq
}

require_password() {
    [[ -n ${LGTM_WEB_PASSWORD:-} ]] || fail "LGTM_WEB_PASSWORD is missing."
    export LGTM_WEB_PASSWORD
}

rpc() {
    local method=$1 data=$2 cookie_file=${3:-} response
    shift 2
    if (($#)); then shift; fi
    local -a cookies=()
    [[ -z $cookie_file ]] || cookies=(--cookie "$cookie_file")
    response=$(curl --silent --noproxy '*' --max-time 5 "${cookies[@]}" "$@" \
        --header 'Content-Type: application/json' --header 'Connect-Protocol-Version: 1' \
        --data-binary @- --write-out $'\n%{http_code}' "$web_url/lgtm.v1.$method" <<< "$data") ||
        return 1
    http_code=${response##*$'\n'}
    body=${response%$'\n'*}
}

get_server() {
    rpc SessionService/GetServer '{}' "${1:-}" && [[ $http_code == 200 ]] &&
        jq -e '.product == "lgtm-studio" and (.contract > 0) and (.sample != true)' \
            <<< "$body" >/dev/null 2>&1
}

await_server() {
    local deadline=$((SECONDS + 60))
    until get_server; do
        ((SECONDS < deadline)) || return 1
        pause
    done
}

pause() {
    local timer
    # Keeping both ends of this empty pipe open makes read wait for its timeout.
    exec {timer}<> <(:)
    read -r -t 1 -u "$timer" _ || :
    exec {timer}>&-
}

signin() {
    local data
    data=$(jq -nc --arg password "$1" '{password: $password}') || return 1
    rpc SessionService/SignIn "$data" '' --cookie-jar "$2" && [[ $http_code == 200 ]] &&
        jq -e '.result == "SIGN_IN_RESULT_SIGNED_IN" or .result == 1' <<< "$body" >/dev/null &&
        get_server "$2" && jq -e '.signedIn == true' <<< "$body" >/dev/null
}

signin_rejected() {
    local data
    data=$(jq -nc --arg password "$1" '{password: $password}') || return 1
    rpc SessionService/SignIn "$data" && [[ $http_code == 200 ]] &&
        jq -e '.result == "SIGN_IN_RESULT_REJECTED" or .result == 2' <<< "$body" >/dev/null
}

unauthenticated() {
    rpc SessionService/SignOut '{}' "${1:-}" && [[ $http_code == 401 ]] &&
        jq -e '.code == "unauthenticated"' <<< "$body" >/dev/null
}

stop_client() {
    if [[ -n $client_pid ]]; then
        kill -KILL "$client_pid" 2>/dev/null || :
        wait "$client_pid" 2>/dev/null || :
        client_pid=''
    fi
}

close_watch() {
    if [[ -n $watch_fd ]]; then
        exec {watch_fd}<&-
        kill "$watch_pid" 2>/dev/null || :
        wait "$watch_pid" 2>/dev/null || :
        watch_fd=''
    fi
}

cleanup() {
    local status=$? id data
    close_watch
    stop_client
    if [[ -n $restore_service ]]; then
        if ! compose up -d --no-deps "${restore_flags[@]}" "$restore_service" >/dev/null 2>&1 ||
            ! await_server; then
            printf '%s\n' "Recovery could not bring $restore_service back online."
            status=1
        fi
    fi
    for id in "${fixture_ids[@]}"; do
        data=$(jq -nc --arg id "$id" '{id: $id}')
        rpc ClientService/RemoveClient "$data" "$jar" >/dev/null 2>&1 || :
    done
    for id in "${fixture_tokens[@]}"; do
        data=$(jq -nc --arg id "$id" '{tokenId: $id}')
        rpc ClientService/RevokeClientToken "$data" "$jar" >/dev/null 2>&1 || :
    done
    if [[ -f $jar ]]; then
        rpc SessionService/SignOut '{}' "$jar" >/dev/null 2>&1 || :
    fi
    if [[ -n $scratch_helper ]] && ! go run "$scratch_helper" "$scratch" "$scratch_helper" >/dev/null 2>&1; then
        printf '%s\n' 'The temporary fixture files could not be removed.'
        status=1
    fi
    exit "$status"
}

make_scratch() {
    local helper=${TMPDIR:-/tmp}/lgtm-wave1-$BASHPID-$RANDOM.go
    need go
    [[ $helper == /* ]] || helper=$PWD/$helper
    # Use Go's temporary-directory functions so the check needs no filesystem utilities.
    (umask 077; set -o noclobber; printf '%s\n' "$scratch_program" > "$helper") ||
        fail "The temporary directory helper could not be written."
    scratch_helper=$helper
    scratch=$(go run "$scratch_helper") ||
        fail "A temporary directory could not be created."
    jar=$scratch/session.cookies
}

read -r -d '' scratch_program <<'GO' || :
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) == 1 {
		dir, err := os.MkdirTemp("", "lgtm-wave1-")
		if err != nil {
			os.Exit(1)
		}
		fmt.Println(dir)
		return
	}
	if os.Args[1] != "" && os.RemoveAll(os.Args[1]) != nil {
		os.Exit(1)
	}
	if os.Remove(os.Args[2]) != nil {
		os.Exit(1)
	}
}
GO

web_session() {
    require_stack
    configure_web
    require_password
    get_server || fail "The Go server did not answer GetServer through the website."
    make_scratch
    signin "$LGTM_WEB_PASSWORD" "$jar" || fail "The configured password did not create a session."
}

machines() {
    [[ -f go.mod ]] || fail "The root Go module is missing."
    [[ -d $client_package ]] || fail "The lgtm-client command package is missing."
    need go
    web_session
    go build -o "$scratch/lgtm-client" "$client_package" >"$scratch/build.log" 2>&1 ||
        fail "The lgtm-client command did not build."
    client_bin=$scratch/lgtm-client
}

new_token() {
    rpc ClientService/CreateClientToken '{}' "$jar" && [[ $http_code == 200 ]] ||
        fail "The Machines page could not create a join token."
    token=$(jq -er '.token | strings | select(length > 0)' <<< "$body" 2>/dev/null) &&
        token_id=$(jq -er '.tokenId | strings | select(length > 0)' <<< "$body" 2>/dev/null) ||
        fail "CreateClientToken returned no token or token identifier."
    fixture_tokens+=("$token_id")
    token_expiry=$(jq -er '.expiresAt | strings | select(length > 0)' <<< "$body" 2>/dev/null) ||
        fail "CreateClientToken returned no expiry."
}

client_join() {
    local pid deadline=$((SECONDS + 15))
    XDG_STATE_HOME=$1 "$client_bin" join "$web_url" "$token" >"$scratch/join.log" 2>&1 &
    pid=$!
    while kill -0 "$pid" 2>/dev/null; do
        if ((SECONDS >= deadline)); then
            kill -KILL "$pid" 2>/dev/null || :
            wait "$pid" 2>/dev/null || :
            fail "lgtm-client join did not finish within 15 seconds."
        fi
        pause
    done
    wait "$pid"
}

start_client() {
    XDG_STATE_HOME=$state_home "$client_bin" run >"$scratch/client.log" 2>&1 &
    client_pid=$!
}

# Connect uses an uncompressed five-byte header before each streaming message.
open_watch() {
    close_watch
    exec {watch_fd}< <(
        curl --silent --noproxy '*' --max-time 240 --no-buffer --cookie "$jar" \
            --header 'Content-Type: application/connect+json' \
            --header 'Connect-Protocol-Version: 1' --data-binary @- \
            "$web_url/lgtm.v1.ClientService/WatchClients" < <(printf '\000\000\000\000\002{}')
    )
    watch_pid=$!
    read_frame "$((SECONDS + 5))" &&
        jq -e '.all | type == "object"' <<< "$frame" >/dev/null 2>&1
}

read_frame() {
    local deadline=$1 flags=0 size=0 octet byte i remaining
    for ((i = 0; i < 5; i++)); do
        remaining=$((deadline - SECONDS))
        ((remaining > 0)) || return 1
        # An empty delimiter makes Bash return the zero byte as an empty string.
        IFS= read -r -d '' -n 1 -t "$remaining" -u "$watch_fd" octet || return 1
        byte=0
        if [[ -n $octet ]]; then
            printf -v byte '%d' "'$octet"
        fi
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

wait_saved() {
    local deadline=$((SECONDS + $1)) predicate=$2
    shift 2
    while read_frame "$deadline"; do
        if record=$(jq -ce "$@" ".saved | select($predicate)" <<< "$frame" 2>/dev/null); then
            return 0
        fi
    done
    return 1
}

online='(.state == "CLIENT_STATE_ONLINE" or .state == 3)'
offline='(.state == "CLIENT_STATE_OFFLINE" or .state == 4)'
waiting='(.state == "CLIENT_STATE_WAITING_FOR_APPROVAL" or .state == 1)'

watch_join() {
    state_home=$scratch/state-$1
    new_token
    open_watch || fail "WatchClients did not send its initial machine list."
    client_join "$state_home" || fail "lgtm-client join refused a fresh token."
    wait_saved 15 "$waiting and .tokenId == \$token" --arg token "$token_id" ||
        fail "The open WatchClients stream did not announce the joined machine as waiting for approval."
    machine_id=$(jq -er '.id | strings | select(length > 0)' <<< "$record" 2>/dev/null) ||
        fail "The waiting machine had no identifier."
    fixture_ids+=("$machine_id")
}

approve_client() {
    local data
    machine_name="Wave 1 $machine_id"
    data=$(jq -nc --arg id "$machine_id" --arg name "$machine_name" '{id: $id, name: $name}')
    rpc ClientService/ApproveClient "$data" "$jar" && [[ $http_code == 200 ]] ||
        fail "The Machines page could not approve the waiting machine."
}

wait_online() {
    wait_saved "$1" ".id == \$id and $online" --arg id "$machine_id"
}

approve_and_run() {
    watch_join "$1"
    approve_client
    start_client
    wait_online 35 || fail "The approved machine did not come online on WatchClients."
}

running_machine() {
    machines
    approve_and_run "$1"
}

psql_in_stack() {
    compose exec -T --user 70:70 postgres sh -c '
        PGPASSWORD=$(cat /run/lgtm-db/postgres-app.password) exec psql -w --no-psqlrc --quiet --tuples-only --no-align \
            -h 127.0.0.1 -U lgtm_app -d lgtm_app --set ON_ERROR_STOP=1 "$@"' sh "$@"
}

log_has() {
    local file=$1 message=$2 code=$3
    jq -Ren --arg msg "$message" --arg code "$code" \
        'any(inputs | fromjson?; .msg == $msg and .code == $code)' "$file" >/dev/null 2>&1
}

refused_join() {
    if client_join "$1"; then
        fail "lgtm-client joined with a $2 token."
    fi
    log_has "$scratch/join.log" join_refused "token_$2" ||
        fail "lgtm-client did not report that the join token was $2."
}

wait_refusal() {
    local deadline=$((SECONDS + 20))
    until log_has "$scratch/client.log" link_refused "$1"; do
        if ((SECONDS >= deadline)) || ! kill -0 "$client_pid" 2>/dev/null; then
            log_has "$scratch/client.log" link_refused "$1"
            return
        fi
        pause
    done
}

check_c1() {
    local page
    [[ -x $up_file ]] || fail "$up_file is missing or is not executable."
    require_stack
    configure_web
    require_password
    "$up_file" >/dev/null 2>&1 || fail "$up_file could not start the stack."
    await_server || fail "The website address did not answer with the Go server."
    page=$(curl --silent --fail --noproxy '*' --max-time 5 "$web_url/") ||
        fail "The loopback address did not serve the website."
    [[ $page == *'id="app"'* && $page == *'<script'* ]] ||
        fail "The loopback address returned no website entry point."
    printf '%s\n' "One command started the stack and opened the website on loopback."
}

check_c2() {
    local state logs deadline
    require_stack
    configure_web
    require_password
    get_server || fail "The Go server was unavailable before the missing-password check."
    restore_service=server
    restore_flags=(--force-recreate)
    docker compose -f "$compose_file" -f - up -d --force-recreate --no-deps server \
        >/dev/null 2>&1 <<'YAML' || fail "Compose could not create the server without a password."
services:
  server:
    restart: "no"
    environment:
      LGTM_WEB_PASSWORD: !reset null
YAML
    deadline=$((SECONDS + 20))
    while :; do
        state=$(compose ps -a --format '{{.State}} {{.ExitCode}}' server 2>/dev/null) ||
            fail "Compose could not report the server's exit status."
        [[ $state == 'exited '* ]] && break
        ((SECONDS < deadline)) || fail "The server did not exit without LGTM_WEB_PASSWORD."
        pause
    done
    [[ $state != 'exited 0' ]] || fail "The server exited successfully without LGTM_WEB_PASSWORD."
    logs=$(compose logs --no-color --no-log-prefix server 2>/dev/null) ||
        fail "Compose could not read the server's startup diagnostic."
    [[ $logs == *LGTM_WEB_PASSWORD* ]] || fail "The startup diagnostic did not name LGTM_WEB_PASSWORD."
    printf '%s\n' "The server refused to start without LGTM_WEB_PASSWORD."
}

check_c3() {
    web_session
    signin_rejected "$LGTM_WEB_PASSWORD-$BASHPID-wrong" || fail "SignIn did not refuse a wrong password."
    unauthenticated || fail "SignOut without a session did not return unauthenticated."
    printf '%s\n' "The server accepted the right password, refused a wrong password, and required a session."
}

check_c4() {
    local changed_password
    web_session
    restore_service=server
    compose restart server >/dev/null 2>&1 || fail "Compose could not restart the server."
    await_server || fail "The server did not return after its restart."
    get_server "$jar" && jq -e '.signedIn == true' <<< "$body" >/dev/null ||
        fail "The original session stopped working after a server restart."
    changed_password=$LGTM_WEB_PASSWORD-$BASHPID-changed
    restore_flags=(--force-recreate)
    LGTM_WEB_PASSWORD="$changed_password" compose up -d --force-recreate --no-deps server >/dev/null 2>&1 ||
        fail "Compose could not start the server with the changed password."
    await_server || fail "The server did not return with the changed password."
    unauthenticated "$jar" || fail "The original session still worked after the password changed."
    signin "$changed_password" "$scratch/changed.cookies" || fail "The changed password could not sign in."
    printf '%s\n' "The original session survived a restart and ended when the password changed."
}

check_c5() {
    local expired
    machines
    watch_join used
    refused_join "$scratch/reuse" used
    new_token
    jq -en --arg expiry "$token_expiry" '
        ($expiry | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601) - now | . >= 885 and . <= 905
    ' >/dev/null 2>&1 || fail "The new join token did not have a 15-minute lifetime."
    expired=$(psql_in_stack --set "id=$token_id" 2>/dev/null <<'SQL'
UPDATE client_join_tokens
SET expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE id = :'id'
RETURNING id;
SQL
    ) || fail "PostgreSQL could not set the fixture token's expiry in the past."
    [[ $expired == "$token_id" ]] || fail "PostgreSQL found no matching join token to expire."
    refused_join "$scratch/expired" expired
    printf '%s\n' "The Machines page made a token, and the server refused used and expired tokens."
}

check_c6() {
    machines
    watch_join pending
    printf '%s\n' "The open WatchClients stream showed the joined machine waiting for approval."
}

check_c7() {
    machines
    watch_join facts
    approve_client
    start_client
    wait_saved 35 "
        .id == \$id and .name == \$name and $online and
        ([.os, .version, .workspace] | all(type == \"string\" and length > 0)) and
        ([.cores, .memoryGigabytes, .agentLimit] | all(type == \"number\" and . > 0)) and
        ([.cpuPercent, .memoryPercent] | all(type == \"number\" and . >= 0 and . <= 100)) and
        .harnessesScanned == true and
        ([.harnesses[].harness] | sort) == [\"claude\", \"codex\", \"cursor\", \"gemini\"] and
        all(.harnesses[]; .version == null or (.version | type == \"string\" and length > 0))
    " --arg id "$machine_id" --arg name "$machine_name" ||
        fail "WatchClients did not show the approved machine online with its facts, load, and four harness results."
    printf '%s\n' "The Machines page showed the approved machine online with its facts, load, and harness results."
}

check_c8() {
    local deadline remaining
    running_machine stop
    deadline=$((SECONDS + 30))
    stop_client
    remaining=$((deadline - SECONDS))
    wait_saved "$remaining" ".id == \$id and $offline" --arg id "$machine_id" ||
        fail "The open WatchClients stream did not show the stopped machine offline within 30 seconds."
    start_client
    wait_online 35 || fail "The open WatchClients stream did not show the restarted machine online."
    printf '%s\n' "The machine went offline within 30 seconds and came online when the client restarted."
}

check_c9() {
    local previous deadline
    running_machine reconnect
    previous=$(jq -er '.connectedAt | strings | select(length > 0)' <<< "$record" 2>/dev/null) ||
        fail "WatchClients did not report the original connection time."
    restore_service=server
    compose restart server >/dev/null 2>&1 || fail "Compose could not restart the server."
    await_server || fail "The server did not answer after its restart."
    deadline=$((SECONDS + 35))
    while ((SECONDS < deadline)); do
        if open_watch && jq -e --arg id "$machine_id" --arg previous "$previous" "
            any(.all.clients[]?; .id == \$id and $online and
                (.connectedAt | type == \"string\" and length > 0 and . != \$previous))
        " <<< "$frame" >/dev/null 2>&1; then
            printf '%s\n' "The same client came online on a new connection after the server restarted."
            return 0
        fi
        pause
    done
    fail "WatchClients did not show a new connection for the client that kept running."
}

check_c10() {
    local data identity replacement
    running_machine unknown
    stop_client
    data=$(jq -nc --arg id "$machine_id" '{id: $id}')
    rpc ClientService/RemoveClient "$data" "$jar" && [[ $http_code == 200 ]] ||
        fail "The server did not remove the machine and forget its key."
    start_client
    wait_refusal unknown_machine || fail "The client did not report the server's refusal of the removed identity."
    stop_client

    approve_and_run wrong-key
    stop_client
    wait_saved 30 ".id == \$id and $offline" --arg id "$machine_id" ||
        fail "The second fixture did not go offline before its saved server key changed."
    identity=$state_home/lgtm-client/identity.json
    [[ -f $identity ]] || fail "The enrolled fixture has no identity.json with a saved server key."
    replacement=$(jq -ce '
        select((.clientPublicKey | type == "string" and length > 0) and
            (.serverPublicKey | type == "string" and length > 0) and .clientPublicKey != .serverPublicKey) |
        .serverPublicKey = .clientPublicKey
    ' "$identity" 2>/dev/null) || fail "The fixture did not have two distinct public keys."
    printf '%s\n' "$replacement" >"$identity" || fail "The fixture's saved server key could not be replaced."
    start_client
    wait_refusal server_key_mismatch || fail "The client did not report refusal of a server with a different key."
    printf '%s\n' "The server refused a removed machine, and the client refused a mismatched server key."
}

client_sequences() {
    jq -Rsc '
        [split("\n")[:-1][] | fromjson? |
         select(.msg == "link_event_buffered" and (.sequence | type == "number")) | .sequence]
    ' "$scratch/client.log"
}

server_sequences() {
    compose logs --no-color --no-log-prefix server 2>/dev/null |
        jq -Rsc --arg id "$machine_id" '
            [split("\n")[] | fromjson? |
             select(.msg == "link_event_applied" and .machine_id == $id and
                 (.sequence | type == "number")) | .sequence]
        '
}

check_c11() {
    local baseline expected applied last deadline
    running_machine replay
    restore_service=server
    compose stop server >/dev/null 2>&1 || fail "Compose could not stop the server for the outage."
    baseline=$(client_sequences | jq -e length) || fail "The client event buffer log could not be read."
    deadline=$((SECONDS + 50))
    while :; do
        expected=$(client_sequences | jq -c --argjson baseline "$baseline" '.[$baseline:]') ||
            fail "The client event buffer log could not be read during the outage."
        if jq -e 'length >= 3' <<< "$expected" >/dev/null; then
            break
        fi
        ((SECONDS < deadline)) || fail "The client did not log at least three durable events while the server was stopped."
        pause
    done
    kill -STOP "$client_pid" 2>/dev/null || fail "The client exited during the outage."
    expected=$(client_sequences | jq -c --argjson baseline "$baseline" '.[$baseline:]') ||
        fail "The final list of events buffered during the outage could not be read."
    jq -e '. == (sort | unique)' <<< "$expected" >/dev/null ||
        fail "The events buffered during the outage were not numbered in strictly increasing order."
    last=$(jq -er 'last' <<< "$expected")
    compose up -d --no-deps server >/dev/null 2>&1 || fail "Compose could not bring the server back."
    await_server || fail "The server did not answer after the outage."
    kill -CONT "$client_pid" 2>/dev/null || fail "The original client could not resume after the outage."
    deadline=$((SECONDS + 45))
    while :; do
        applied=$(server_sequences) || fail "The server event application log could not be read."
        if jq -e --argjson last "$last" 'any(.[]; . > $last)' <<< "$applied" >/dev/null; then
            break
        fi
        ((SECONDS < deadline)) || fail "The server never applied a later event after recovery."
        pause
    done
    jq -e --argjson expected "$expected" '
        [.[] | select(. as $number | $expected | index($number))] == $expected
    ' <<< "$applied" >/dev/null ||
        fail "The server event application log showed missing, repeated, or out-of-order buffered events."
    printf '%s\n' "The events buffered during the outage were applied once each and in order."
}

check_c12() {
    if [[ ! -f go.mod && ! -x $mutation_hook ]]; then
        fail "The root Go module and executable $mutation_hook are missing."
    fi
    [[ -f go.mod ]] || fail "The root Go module is missing."
    need go
    [[ -n ${LGTM_TEST_POSTGRES:-} ]] || fail "LGTM_TEST_POSTGRES is missing for the real PostgreSQL tests."
    export LGTM_TEST_POSTGRES
    go test -count=1 ./... >/dev/null 2>&1 || fail "go test ./... failed against the configured PostgreSQL."
    [[ -x $mutation_hook ]] || fail "The executable $mutation_hook is missing."
    "$mutation_hook" >/dev/null 2>&1 || fail "The mutation hook did not confirm that removing the lock fails the feed test."
    printf '%s\n' "The Go tests passed against PostgreSQL, and the feed test failed with the lock removed."
}

vault_ready() {
    local result
    result=$(compose exec -T openbao bao status -format=json 2>/dev/null) &&
        jq -e '.initialized == true and .sealed == false' <<< "$result" >/dev/null 2>&1
}

server_key_fingerprint() {
    compose logs --no-color --no-log-prefix server 2>/dev/null |
        jq -Rser '
            [split("\n")[] | fromjson? |
             select(.msg == "link_signing_key_loaded" and .source == "openbao") |
             .fingerprint | strings | select(length > 0)] | last // empty
        '
}

check_c13() {
    local running original='' after deadline
    local -a services
    require_stack
    need jq
    running=$(compose ps --services --status running 2>/dev/null) ||
        fail "Compose could not list the running services."
    mapfile -t services <<< "$running"
    [[ $'\n'$running$'\n' == *$'\nopenbao\n'* && $'\n'$running$'\n' == *$'\npostgres\n'* ]] ||
        fail "The postgres and openbao services were not both running before the restart."
    vault_ready || fail "OpenBao was not initialized and unsealed before the restart."
    if [[ $'\n'$running$'\n' == *$'\nserver\n'* ]]; then
        original=$(server_key_fingerprint) || original=''
    fi
    if ! compose up -d --force-recreate "${services[@]}" >/dev/null 2>&1; then
        if ! compose up -d "${services[@]}" >/dev/null 2>&1; then
            fail "Compose failed to recreate the stack and could not bring it back up."
        fi
        fail "Compose failed to recreate the stack."
    fi
    deadline=$((SECONDS + 60))
    until vault_ready; do
        ((SECONDS < deadline)) || fail "OpenBao stayed sealed after the stack restarted."
        pause
    done
    [[ -n $original ]] || fail "OpenBao unsealed after restart, but the original server signing key load was missing."
    while :; do
        if after=$(server_key_fingerprint); then
            [[ $after == "$original" ]] || fail "The restarted server loaded a different signing key from OpenBao."
            break
        fi
        ((SECONDS < deadline)) || fail "OpenBao unsealed, but the restarted server did not report loading its signing key."
        pause
    done
    configure_web
    await_server || fail "The server loaded its signing key but did not answer through the restarted website."
    printf '%s\n' "OpenBao unsealed without input and the restarted server loaded the same signing key from it."
}

run_check() {
    local number=$1 observation
    if observation=$(
        scratch='' scratch_helper='' jar='' client_pid='' watch_fd='' watch_pid='' restore_service=''
        fixture_ids=() fixture_tokens=() restore_flags=()
        trap cleanup EXIT
        "check_c$number" 2>/dev/null
    ); then
        printf 'ok %s - C%s %s\n' "$number" "$number" "$observation"
    else
        printf 'not ok %s - C%s %s\n' "$number" "$number" "${observation//$'\n'/ }"
        return 1
    fi
}

usage() {
    printf 'Usage: %s [--only C1,C12]\n' "${0##*/}" >&2
    exit 2
}

checks=(1 2 3 4 5 6 7 8 9 10 11 12 13)
if (($#)); then
    [[ $# == 2 && $1 == --only && $2 =~ ^C([1-9]|1[0-3])(,C([1-9]|1[0-3]))*$ ]] || usage
    requested=,$2,
    checks=()
    for ((number = 1; number <= 13; number++)); do
        [[ $requested != *",C$number,"* ]] || checks+=("$number")
    done
fi

result=0
for number in "${checks[@]}"; do
    run_check "$number" || result=1
done
exit "$result"
