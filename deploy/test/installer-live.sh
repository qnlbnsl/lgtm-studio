#!/usr/bin/env bash
set +x
if [[ ${LGTM_INSTALL_LIVE:-} != 1 ]]; then
    printf '%s\n' '1..8'
    exit 2
fi

set +a
set -uo pipefail
umask 077
export LC_ALL=C
export -n LGTM_WEB_PASSWORD

case "$#:${1:-}" in
    0: | 1:--after-reboot | 1:--cleanup) ;;
    *)
        printf '%s\n' 'Usage: deploy/test/installer-live.sh [--after-reboot | --cleanup]' >&2
        exit 2
        ;;
esac
part=${1:-}
started=$SECONDS

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P) 2>/dev/null &&
    cd -- "$repo" 2>/dev/null || fail 'The repository could not be opened.'
[[ ${HOME:-} == /* && $HOME != / && $EUID != 0 && $(uname -s) == Linux ]] ||
    fail 'Run the live test as a disposable Linux user.'

fixture=$HOME/.local/state/lgtm-installer-live
state=$fixture/state
identity=$state/identity.json
unit=lgtm-client.service
unit_file=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/$unit
bin=$HOME/.local/bin/lgtm-client
installer=$PWD/deploy/install-client.sh
jar='' watch_fd='' watch_pid='' web_url=''
boot_file=/proc/sys/kernel/random/boot_id

present() {
    [[ -e $1 || -L $1 ]]
}

private_directory() {
    [[ -d $1 && ! -L $1 && -O $1 ]] && [[ $(stat -c '%a' -- "$1" 2>/dev/null) == 700 ]]
}

private_file() {
    [[ -f $1 && ! -L $1 && -O $1 ]] && [[ $(stat -c '%a' -- "$1" 2>/dev/null) == 600 ]]
}

lock_fixture() {
    need flock
    private_directory "$fixture" && exec {fixture_lock}<"$fixture" &&
        flock -x "$fixture_lock" || fail 'The private live fixture could not be locked.'
    # The open directory could have been removed while this invocation waited.
    if ! present "$fixture"; then
        return 1
    fi
    [[ $fixture -ef /dev/fd/$fixture_lock && ! -L $state ]] ||
        fail 'The live fixture changed while acquiring its lock.'
}

unit_property() {
    systemctl --user show --property="$1" --value "$unit" 2>/dev/null
}

read_unit_load() {
    unit_load=$(unit_property LoadState) || [[ $unit_load == not-found ]] ||
        fail 'The user unit load state could not be read.'
}

unit_owned_by_fixture() {
    local environment
    if present "$unit_file"; then
        [[ -f $unit_file && ! -L $unit_file ]] &&
            [[ $(sed -n 's/^Environment=LGTM_CLIENT_STATE_DIR=//p' "$unit_file" 2>/dev/null) == "$state" ]] ||
            return 1
    fi
    if [[ $unit_load != not-found ]]; then
        environment=$(unit_property Environment) || return 1
        [[ " $environment " == *" LGTM_CLIENT_STATE_DIR=$state "* ]] || return 1
    fi
}

read_address() {
    local published port
    if [[ -n ${COMPOSE_FILE:-} ]]; then
        published=$(docker compose port web 8080 2>/dev/null && printf '.') || return 1
    else
        published=$(docker compose -f "$PWD/deploy/compose.yaml" port web 8080 2>/dev/null && printf '.') ||
            return 1
    fi
    published=${published%.}
    published=${published%$'\n'}
    port=${published#127.0.0.1:}
    [[ $published == 127.0.0.1:* && $port =~ ^[0-9]{1,5}$ ]] &&
        ((10#$port >= 1 && 10#$port <= 65535)) || return 1
    web_url=http://$published
}

rpc() {
    local method=$1 response
    shift
    response=$(curl --disable --silent --noproxy '*' --max-time 5 "$@" \
        --header 'Content-Type: application/json' --header 'Connect-Protocol-Version: 1' \
        --data-binary @- --write-out $'\n%{http_code}' "$web_url/lgtm.v1.$method" 2>/dev/null) ||
        return 1
    http_code=${response##*$'\n'}
    body=${response%$'\n'*}
}

get_server() {
    rpc SessionService/GetServer "$@" <<< '{}' && [[ $http_code == 200 ]] &&
        jq -e -s 'length == 1 and (.[0] | .product == "lgtm-studio"
            and (.contract | type == "number" and . > 0) and .sample != true)' \
            <<< "$body" >/dev/null 2>&1
}

signin() {
    [[ -n ${LGTM_WEB_PASSWORD:-} ]] || return 1
    jar=$(mktemp -- "$fixture/session.XXXXXX" 2>/dev/null) || return 1
    rpc SessionService/SignIn --cookie-jar "$jar" < <(
        printf '%s' "$LGTM_WEB_PASSWORD" | jq -Rsc '{password: .}' 2>/dev/null
    ) && [[ $http_code == 200 ]] &&
        jq -e '.result == "SIGN_IN_RESULT_SIGNED_IN" or .result == 1' <<< "$body" >/dev/null 2>&1 &&
        get_server --cookie "$jar" && jq -e '.signedIn == true' <<< "$body" >/dev/null 2>&1
}

revoke_session() {
    [[ ! -s $1 ]] ||
        { rpc SessionService/SignOut --cookie "$1" <<< '{}' && [[ $http_code == 200 || $http_code == 401 ]]; }
}

signout() {
    [[ -n $jar ]] || return 0
    revoke_session "$jar" || return 1
    rm -f -- "$jar" 2>/dev/null || return 1
    jar=''
}

close_watch() {
    if [[ -n $watch_fd ]]; then
        exec {watch_fd}<&-
        kill "$watch_pid" 2>/dev/null || :
        wait "$watch_pid" 2>/dev/null || :
        watch_fd=''
    fi
}

on_exit() {
    local status=$?
    trap '' HUP INT TERM
    close_watch
    if ! signout; then
        printf '%s\n' 'The live test could not sign out. Its cookie jar remains private in the fixture.' >&2
        ((status != 0)) || status=1
    fi
    exit "$status"
}

trap on_exit EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# Connect uses an uncompressed five-byte header before each streaming message.
open_watch() {
    local deadline=$1 remaining
    close_watch
    remaining=$((deadline - SECONDS))
    ((remaining > 0)) || return 1
    exec {watch_fd}< <(
        exec curl --disable --silent --noproxy '*' --max-time "$remaining" --no-buffer --cookie "$jar" \
            --header 'Content-Type: application/connect+json' --header 'Connect-Protocol-Version: 1' \
            --data-binary @- "$web_url/lgtm.v1.ClientService/WatchClients" \
            < <(printf '\000\000\000\000\002{}') 2>/dev/null
    )
    watch_pid=$!
    read_frame "$deadline" && jq -e '.all | type == "object"' <<< "$frame" >/dev/null 2>&1
}

read_frame() {
    local deadline=$1 flags=0 size=0 octet byte i remaining
    for ((i = 0; i < 5; i++)); do
        remaining=$((deadline - SECONDS))
        ((remaining > 0)) || return 1
        # An empty delimiter makes Bash read a zero byte as an empty string.
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

snapshot() {
    local status=0
    open_watch "$1" || status=$?
    close_watch
    return "$status"
}

waiting='(.state == "CLIENT_STATE_WAITING_FOR_APPROVAL" or .state == 1)'
online='(.state == "CLIENT_STATE_ONLINE" or .state == 3)'

connection_time() {
    local stamp
    stamp=$(jq -er --arg id "$machine_id" \
        "(.all.clients[]?, .saved) | select(.id == \$id and $online) | .connectedAt | strings" \
        <<< "$frame" 2>/dev/null) || return 1
    [[ $stamp == *Z ]] || return 1
    date -u -d "$stamp" '+%Y%m%d%H%M%S%N' 2>/dev/null
}

wait_online() {
    local deadline=$1 previous=${2:-} observed
    open_watch "$deadline" || return 1
    while true; do
        if jq -e --arg id "$machine_id" \
            "any(.all.clients[]?, .saved; .id == \$id and $online and .hostsServer == true)" \
            <<< "$frame" >/dev/null 2>&1; then
            if [[ -z $previous ]] || { observed=$(connection_time) && [[ $observed > $previous ]]; }; then
                close_watch
                return 0
            fi
        fi
        read_frame "$deadline" || return 1
    done
}

read_identity() {
    [[ -f $identity && ! -L $identity ]] &&
        machine_id=$(jq -er '.machineId | strings' "$identity" 2>/dev/null) &&
        [[ $machine_id =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]
}

write_record() {
    local record
    record=$(mktemp -- "$fixture/record.XXXXXX" 2>/dev/null) &&
        printf '%s\0' "$boot_id" "$web_url" "${COMPOSE_FILE:-}" "${XDG_CONFIG_HOME:-}" > "$record" &&
        mv -T -- "$record" "$fixture/fixture"
}

read_record() {
    local -a record
    private_file "$fixture/fixture" && mapfile -d '' -t record < "$fixture/fixture" &&
        [[ ${#record[@]} == 3 || ${#record[@]} == 4 ]] || return 1
    recorded_boot=${record[0]}
    recorded_address=${record[1]}
    if [[ -n ${record[2]} ]]; then
        export COMPOSE_FILE="${record[2]}"
    else
        unset COMPOSE_FILE
    fi
    if [[ ${#record[@]} == 4 ]]; then
        if [[ -n ${record[3]} ]]; then
            export XDG_CONFIG_HOME="${record[3]}"
        else
            unset XDG_CONFIG_HOME
        fi
        unit_file=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/$unit
    fi
}

read_recorded_address() {
    read_address && [[ $web_url == "$recorded_address" ]]
}

fixture_install() (
    # The installer inherits the lock descriptor and these two functions.
    # Each registration reserves the exact staging path before anything is created beside the target.
    export live_fixture="$fixture" live_bin="$bin" live_unit_file="$unit_file"
    mktemp() {
        local target kind registration staging
        [[ $# == 2 && $1 == -- ]] || return 1
        case $2 in
            "$live_bin".XXXXXX) target=$live_bin; kind=bin ;;
            "$live_unit_file".XXXXXX) target=$live_unit_file; kind=unit ;;
            *) return 1 ;;
        esac
        registration=$(command mktemp -- "$live_fixture/$kind.XXXXXX") || return 1
        staging=$target.fixture-${registration##*.}
        mkdir -m 0700 -- "$staging" && (set -C; : > "$staging/pending") || return 1
        printf '%s\n' "$staging/pending"
    }
    mv() {
        local source=${@: -2:1} target=${@: -1}
        if [[ $target == "$live_bin" ]]; then
            # The hard link proves ownership even if the installer dies just after the rename.
            [[ $source == "$live_bin".fixture-*/pending ]] &&
                ln -- "$source" "${source%/*}/program" || return 1
        fi
        command mv "$@"
    }
    export -f mktemp mv
    exec "$installer"
)

locate_staging() {
    local registration=$1 target=$2 suffix
    suffix=${registration##*.}
    private_file "$registration" && [[ $suffix =~ ^[a-zA-Z0-9]{6}$ ]] ||
        fail 'The fixture staging record could not be verified.'
    staging=$target.fixture-$suffix
    if present "$staging"; then
        private_directory "$staging" || fail 'The fixture staging directory could not be verified.'
    fi
}

remove_staging() {
    local kind=$1 target=$2 registration staging
    for registration in "$fixture/$kind".*; do
        present "$registration" || continue
        locate_staging "$registration" "$target"
        if present "$staging"; then
            rm -f -- "$staging/pending" "$staging/program" && rmdir -- "$staging" ||
                fail 'The installer staging files could not be removed.'
        fi
    done
}

fixture_owns_program() {
    local registration staging
    [[ -f $bin && ! -L $bin ]] || return 1
    for registration in "$fixture"/bin.*; do
        present "$registration" || continue
        locate_staging "$registration" "$bin"
        if [[ -f $staging/program && ! -L $staging/program && $bin -ef $staging/program ]]; then
            return 0
        fi
    done
    return 1
}

machine_rpc() {
    local method=$1
    shift
    rpc "ClientService/$method" --cookie "$jar" < <(
        jq -nc --arg id "$machine_id" "$@" 2>/dev/null
    )
}

reason=''
reject() {
    reason=$1
    return 1
}

live_install_leaves_machine_waiting() {
    local before
    TMPDIR="$fixture" LGTM_INSTALLER_UNDER_TEST="$installer" \
        bash "$repo/deploy/test/installer.sh" --join-transport >/dev/null 2>&1 ||
        { reject 'The installer must pass the join token on standard input.'; return 1; }
    snapshot "$((SECONDS + 5))" ||
        { reject 'WatchClients did not send the machine list before installation.'; return 1; }
    before=$(jq -c '[.all.clients[]?.id]' <<< "$frame" 2>/dev/null) || return 1

    LGTM_CLIENT_STATE_DIR="$state" fixture_install >/dev/null 2>&1 ||
        { reject 'The installer exited unsuccessfully.'; return 1; }
    read_identity || { reject 'The installed client did not write a machine identifier.'; return 1; }
    snapshot "$((SECONDS + 5))" ||
        { reject 'WatchClients did not send the machine list after installation.'; return 1; }
    printf '%s\n%s\n' "$before" "$frame" | jq -es --arg id "$machine_id" "
        . as [\$before, \$after] |
        [\$after.all.clients[]? | select(.id as \$candidate | \$before | index(\$candidate) == null)] |
        length == 1 and (.[0] | .id == \$id and $waiting and .hostsServer == true)
    " >/dev/null 2>&1 ||
        reject 'The first machine list after installation did not show exactly one new machine waiting with hostsServer set.'
}

live_service_enabled_with_linger() {
    [[ $(systemctl --user is-enabled "$unit" 2>/dev/null) == enabled &&
        $(systemctl --user is-active "$unit" 2>/dev/null) == active &&
        $(loginctl show-user "$(id -un)" --property=Linger --value 2>/dev/null) == yes ]] ||
        reject 'The user unit was not enabled and active with lingering enabled.'
}

wait_restarted() {
    local deadline=$1 previous_pid=$2 pid
    while ((SECONDS < deadline)); do
        pid=$(unit_property MainPID) || return 1
        if [[ $pid =~ ^[1-9][0-9]*$ && $pid != "$previous_pid" &&
            $(unit_property ActiveState) == active ]]; then
            return 0
        fi
        sleep 1
    done
    return 1
}

live_service_restarts_after_crash() {
    local pid deadline
    pid=$(unit_property MainPID) && [[ $pid =~ ^[1-9][0-9]*$ ]] ||
        { reject 'The user unit had no main process before the crash.'; return 1; }
    deadline=$((SECONDS + 15))
    systemctl --user kill --signal=KILL "$unit" >/dev/null 2>&1 ||
        { reject 'The service manager could not kill the user unit.'; return 1; }
    wait_restarted "$deadline" "$pid" ||
        reject 'The user unit did not restart with a different main process within 15 seconds.'
}

live_approved_machine_online() {
    local deadline=$((SECONDS + 35))
    machine_rpc ApproveClient --arg name "Installer live $machine_id" '{id: $id, name: $name}' &&
        [[ $http_code == 200 ]] || { reject 'ApproveClient did not approve the fixture machine.'; return 1; }
    wait_online "$deadline" ||
        reject 'WatchClients did not show the approved machine online with hostsServer set within 35 seconds.'
}

journal_has() {
    local messages
    messages=$(journalctl --user --unit "$unit" --since "$1" --no-pager --output=cat 2>/dev/null) ||
        return 1
    grep -Eq -- "$2" <<< "$messages"
}

live_reconnect_restarts_service() {
    local previous pid restarts expected since deadline
    snapshot "$((SECONDS + 5))" && previous=$(connection_time) ||
        { reject 'The machine had no online connection timestamp before ReconnectClient.'; return 1; }
    pid=$(unit_property MainPID) && [[ $pid =~ ^[1-9][0-9]*$ ]] &&
        restarts=$(unit_property NRestarts) && [[ $restarts =~ ^[0-9]+$ ]] ||
        { reject 'The running unit did not expose its main process and automatic restart count.'; return 1; }
    expected=$((restarts + 1))
    since=$(date '+@%s.%6N') || return 1
    deadline=$((SECONDS + 35))
    machine_rpc ReconnectClient '{id: $id}' && [[ $http_code == 200 ]] ||
        { reject 'ReconnectClient did not accept the restart.'; return 1; }
    wait_restarted "$deadline" "$pid" ||
        { reject 'The user unit did not restart with a different main process within 35 seconds.'; return 1; }
    wait_online "$deadline" "$previous" ||
        { reject 'A new WatchClients stream did not show a later online connection within 35 seconds.'; return 1; }
    [[ $(unit_property NRestarts) == "$expected" ]] ||
        { reject 'The automatic restart count did not grow by exactly one.'; return 1; }
    journal_has "$since" '^lgtm-client\.service: Main process exited, code=exited, status=75(/[^[:space:]]+)?[[:space:]]*$' ||
        reject 'The user journal did not record the ended run exiting with status 75.'
}

live_rerun_keeps_machine() {
    local previous count digest deadline
    snapshot "$((SECONDS + 5))" && previous=$(connection_time) ||
        { reject 'The fixture machine was not online before the second install.'; return 1; }
    count=$(jq -e '.all.clients // [] | length' <<< "$frame" 2>/dev/null) &&
        digest=$(sha256sum < "$identity") || return 1

    (unset LGTM_CLIENT_STATE_DIR; fixture_install) >/dev/null 2>&1 ||
        { reject 'The second install exited unsuccessfully.'; return 1; }
    deadline=$((SECONDS + 35))
    wait_online "$deadline" "$previous" ||
        { reject 'The same machine did not come online again within 35 seconds.'; return 1; }
    snapshot "$((SECONDS + 5))" &&
        jq -e --argjson expected "$count" '(.all.clients // [] | length) == $expected' \
            <<< "$frame" >/dev/null 2>&1 ||
        { reject 'The machine count changed or could not be read after the second install.'; return 1; }
    [[ $(sha256sum < "$identity") == "$digest" ]] || reject 'The second install changed identity.json.'
}

live_online_after_reboot() {
    local deadline=$((started + 60))
    [[ $boot_id != "$recorded_boot" ]] ||
        { reject 'The boot identifier did not change.'; return 1; }
    until read_address; do
        ((SECONDS < deadline)) || { reject 'Compose did not report the published port within 60 seconds.'; return 1; }
        sleep 1
    done
    [[ $web_url == "$recorded_address" ]] ||
        { reject 'The published address differs from the recorded address.'; return 1; }
    until get_server; do
        ((SECONDS < deadline)) || { reject 'The server did not answer within 60 seconds.'; return 1; }
        sleep 1
    done
    signin || { reject 'The live test could not sign in after reboot.'; return 1; }
    wait_online "$deadline" ||
        reject 'WatchClients did not show the recorded machine online within 60 seconds.'
}

live_removed_machine_stays_stopped() {
    local deadline=$1 since=$2 restarts
    while ((SECONDS < deadline)); do
        if [[ $(unit_property ActiveState) == inactive && $(unit_property ExecMainCode) == 1 &&
            $(unit_property ExecMainStatus) == 0 ]] &&
            journal_has "$since" '^lgtm-client\.service: Deactivated successfully\.[[:space:]]*$'; then
            break
        fi
        sleep 1
    done
    ((SECONDS < deadline)) ||
        { reject 'The unit did not exit cleanly and become inactive within 35 seconds after removal.'; return 1; }
    restarts=$(unit_property NRestarts) && [[ $restarts =~ ^[0-9]+$ ]] || return 1
    sleep 15
    [[ $(unit_property ActiveState) == inactive && $(unit_property NRestarts) == "$restarts" ]] ||
        reject 'The user unit did not stay inactive for 15 seconds without an automatic restart.'
}

run_test() {
    local number=$1 name=$2
    shift 2
    reason=''
    if "$name" "$@"; then
        printf 'ok %s - %s\n' "$number" "$name"
        return 0
    fi
    close_watch
    printf 'not ok %s - %s %s\n' "$number" "$name" "${reason:-The test could not read its evidence.}"
    return 1
}

need() {
    local tool
    for tool in "$@"; do
        command -v "$tool" >/dev/null 2>&1 || fail "The live test needs $tool."
    done
}

run_default() {
    local status=0
    read_unit_load
    ! present "$fixture" && ! present "$bin" && ! present "$unit_file" && [[ $unit_load == not-found ]] ||
        fail 'The live fixture, client program or user unit already exists.'
    need docker curl jq go loginctl sha256sum journalctl flock
    [[ $(systemctl is-enabled docker.service 2>/dev/null) == enabled ]] ||
        fail 'Docker must be enabled at boot.'
    [[ -n ${LGTM_WEB_PASSWORD:-} ]] || fail 'Set LGTM_WEB_PASSWORD to the fixture website password.'
    read_address && get_server || fail 'The Compose stack did not answer on its published loopback address.'
    mkdir -p -- "${fixture%/*}" && mkdir -m 0700 -- "$fixture" ||
        fail 'The live fixture directory could not be created.'
    lock_fixture || fail 'The new fixture was removed before it could be used.'
    ! present "$bin" && ! present "$unit_file" || fail 'The client program or user unit already exists.'
    write_record || fail 'The live fixture record could not be written.'
    signin || fail 'The live test could not sign in.'

    if ! run_test 1 live_install_leaves_machine_waiting; then
        printf '%s\n' \
            'ok 2 - live_service_enabled_with_linger # SKIP installation check failed' \
            'ok 3 - live_service_restarts_after_crash # SKIP installation check failed' \
            'ok 4 - live_approved_machine_online # SKIP installation check failed' \
            'ok 5 - live_reconnect_restarts_service # SKIP installation check failed' \
            'ok 6 - live_rerun_keeps_machine # SKIP installation check failed'
        return 1
    fi
    run_test 2 live_service_enabled_with_linger || status=1
    run_test 3 live_service_restarts_after_crash || status=1
    if ! run_test 4 live_approved_machine_online; then
        printf '%s\n' \
            'ok 5 - live_reconnect_restarts_service # SKIP approval check failed' \
            'ok 6 - live_rerun_keeps_machine # SKIP approval check failed'
        return 1
    fi
    run_test 5 live_reconnect_restarts_service || status=1
    run_test 6 live_rerun_keeps_machine || status=1
    return "$status"
}

run_cleanup() {
    local status=0 active='' since='' deadline=0 observation='skip'
    local old_jar address_verified=0 uninstall=0
    if ! present "$fixture" || ! lock_fixture; then
        printf '%s\n' 'ok 8 # SKIP no running service'
        return 0
    fi

    if ! present "$fixture/fixture"; then
        ! present "$state" || fail 'The fixture record is missing and the state folder exists.'
        rm -rf -- "$fixture" || fail 'The live fixture directory could not be removed.'
        printf '%s\n' 'ok 8 # SKIP no running service'
        return 0
    fi
    read_record || fail 'The fixture record could not be verified.'
    read_unit_load
    unit_owned_by_fixture || fail 'The user unit does not name the fixture state folder.'
    if fixture_owns_program; then
        uninstall=1
    elif present "$unit_file" || [[ $unit_load != not-found ]]; then
        ! present "$bin" || fail 'The program has no fixture ownership record. Move it aside before cleanup.'
        uninstall=1
    fi

    for old_jar in "$fixture"/session.*; do
        private_file "$old_jar" && [[ -s $old_jar ]] || continue
        if ((address_verified == 0)); then
            read_recorded_address && get_server || break
            address_verified=1
        fi
        revoke_session "$old_jar" ||
            printf '%s\n' 'The live test could not sign out an earlier session.' >&2
    done
    if present "$identity"; then
        need docker curl jq journalctl
        read_identity || fail 'The fixture machine identifier could not be read.'
        ((address_verified)) || read_recorded_address ||
            fail 'The published address could not be verified for machine removal.'
        signin || fail 'The live test could not sign in for removal.'
        if [[ $unit_load != not-found ]]; then
            active=$(unit_property ActiveState) || fail 'The user unit state could not be read.'
        fi
        since=$(date '+@%s.%6N') || fail 'The start time of removal could not be read.'
        deadline=$((SECONDS + 35))
        machine_rpc RemoveClient '{id: $id}' || fail 'RemoveClient could not reach the server.'
        if [[ $http_code != 200 ]]; then
            [[ $http_code == 404 ]] && jq -e '.code == "not_found"' <<< "$body" >/dev/null 2>&1 ||
                fail 'RemoveClient could not remove the fixture machine.'
        fi
        if [[ $active == active ]]; then
            reason=''
            if live_removed_machine_stays_stopped "$deadline" "$since"; then
                observation='pass'
            else
                observation='fail'
                status=1
            fi
        fi
    elif present "$state/machine.key"; then
        observation='fail'
        reason='Remove the waiting machine on the Machines page.'
        status=1
    fi

    if ((uninstall)); then
        "$installer" --remove >/dev/null 2>&1 || fail 'The installer could not remove the user unit and program.'
    fi
    signout || fail 'The live test could not sign out before removing the fixture.'
    remove_staging bin "$bin"
    remove_staging unit "$unit_file"
    rm -rf -- "$fixture" || fail 'The live fixture directory could not be removed.'
    case $observation in
        skip) printf '%s\n' 'ok 8 # SKIP no running service' ;;
        pass) printf '%s\n' 'ok 8 - live_removed_machine_stays_stopped' ;;
        fail) printf 'not ok 8 - live_removed_machine_stays_stopped %s\n' "${reason:-The test could not read its evidence.}" ;;
    esac
    return "$status"
}

systemctl --user show-environment >/dev/null 2>&1 || fail 'The systemd user manager is unavailable.'
case $part in
    '')
        IFS= read -r boot_id < "$boot_file" || fail 'The boot identifier could not be read.'
        run_default
        ;;
    --cleanup) run_cleanup ;;
    --after-reboot)
        lock_fixture || fail 'The live fixture is missing.'
        need docker curl jq
        IFS= read -r boot_id < "$boot_file" || fail 'The boot identifier could not be read.'
        read_record && read_identity || fail 'The fixture record or machine identifier could not be read.'
        read_unit_load
        [[ $unit_load == loaded && -f $unit_file ]] && unit_owned_by_fixture ||
            fail 'The installed user unit does not name the fixture state folder.'
        run_test 7 live_online_after_reboot
        ;;
esac
