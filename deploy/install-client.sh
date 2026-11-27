#!/usr/bin/env bash
set +x
set +a
set -u
export LC_ALL=C

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

compose() {
    if [[ -n ${COMPOSE_FILE:-} ]]; then
        docker compose "$@" 2>/dev/null
    else
        docker compose -f "$repo/deploy/compose.yaml" "$@" 2>/dev/null
    fi
}

get_server() {
    local response body
    response=$(curl --disable --silent --noproxy '*' --max-time 5 \
        --header 'Content-Type: application/json' --header 'Connect-Protocol-Version: 1' \
        --data-binary @- --write-out $'\n%{http_code}' \
        "$address/lgtm.v1.SessionService/GetServer" <<< '{}') || return 1
    [[ ${response##*$'\n'} == 200 ]] || return 1
    body=${response%$'\n'*}
    jq -e -s 'length == 1 and (.[0] | .product == "lgtm-studio"
        and (.contract | type == "number" and . > 0) and .sample != true)' \
        <<< "$body" >/dev/null 2>&1
}

safe_absolute_path() {
    [[ $1 == /* && $1 =~ ^[a-zA-Z0-9._/+@-]+$ ]]
}

service_path() {
    local entry result='' separator='' remaining=${PATH:-}:
    while [[ -n $remaining ]]; do
        entry=${remaining%%:*}
        remaining=${remaining#*:}
        if [[ $entry =~ ^[a-zA-Z0-9._/+@-]*$ ]]; then
            result+=$separator$entry
            separator=:
        fi
    done
    printf '%s' "$result"
}

cleanup() {
    if [[ -n $tmp_bin ]]; then rm -f -- "$tmp_bin"; fi
    if [[ -n $tmp_unit ]]; then rm -f -- "$tmp_unit"; fi
}

replace_file() {
    if [[ $os == Darwin ]]; then
        [[ ! -d $2 ]] && mv -f -- "$1" "$2"
    else
        mv -fT -- "$1" "$2"
    fi
}

manage_unit() {
    systemctl --user "$@" >/dev/null 2>&1 ||
        fail "systemctl --user $1 lgtm-client.service failed."
}

remove_client() {
    if ! systemctl --user disable --now lgtm-client.service >/dev/null 2>&1 &&
        [[ -e $unit_file ]]; then
        fail 'systemctl --user disable lgtm-client.service failed.'
    fi
    rm -f -- "$unit_file" "$bin" 2>/dev/null ||
        fail 'The installed program or user service could not be removed.'
    manage_unit daemon-reload
    printf '%s\n' 'Removed the lgtm-client user service and program and kept its state.'
}

bootout_launch_agent() {
    if ! launchctl bootout "$launch_target" >/dev/null 2>&1 &&
        launchctl print "$launch_target" >/dev/null 2>&1; then
        fail 'launchctl bootout dev.lgtmstudio.client failed.'
    fi
}

remove_launch_agent() {
    bootout_launch_agent
    rm -f -- "$unit_file" "$bin" 2>/dev/null ||
        fail 'The installed program or launch agent could not be removed.'
    printf '%s\n' 'Removed the lgtm-client launch agent and program and kept its state.'
}

launch_agent_state_dir() {
    # Read the state setting from the XML layout written by this installer.
    awk '
        /<key>EnvironmentVariables<\/key>/, /<\/dict>/ {
            if ($0 ~ /^[[:blank:]]*<key>LGTM_CLIENT_STATE_DIR<\/key>[[:blank:]]*$/ && getline > 0) {
                if ($0 ~ /^[[:blank:]]*<string>[^<]*<\/string>[[:blank:]]*$/) {
                    sub(/^[[:blank:]]*<string>/, "")
                    sub(/<\/string>[[:blank:]]*$/, "")
                    print
                }
                exit
            }
        }
    ' "$unit_file"
}

build_version() {
    local commit changes version=0.0.0-dev
    if commit=$(git rev-parse HEAD 2>/dev/null) && [[ $commit =~ ^[0-9a-f]{12,}$ ]]; then
        version=0.0.0-${commit:0:12}
        if changes=$(git --no-optional-locks status --porcelain 2>/dev/null) &&
            [[ -n $changes ]]; then
            version+=-dirty
        fi
    fi
    printf '%s' "$version"
}

join_stack() {
    local token join_output
    export -n token join_output

    # The final dot lets Bash preserve newlines while capturing the command's output.
    token=$(compose exec -T server lgtm-server join-token --hosts-server && printf '.') ||
        fail "The server's join-token command returned no token."
    token=${token%.}
    token=${token%$'\n'}
    [[ $token =~ ^lgtm1_[a-zA-Z0-9_-]+$ ]] ||
        fail "The server's join-token command returned no token."
    unset BASH_REMATCH

    local join_failed=0
    # Bash can put a here-string in a temporary file.
    # Probe with dummy data and keep the line within the minimum atomic pipe write size.
    if (( ${#token} < 512 )) && [[ -p /dev/fd/0 ]] 2>/dev/null <<< "${token//?/x}"; then
        if ! join_output=$(LGTM_CLIENT_STATE_DIR="$state_dir" "$tmp_bin" join "$address" - <<< "$token" 2>&1); then
            join_failed=1
        fi
    elif ! join_output=$(builtin printf '%s\n' "$token" |
        LGTM_CLIENT_STATE_DIR="$state_dir" "$tmp_bin" join "$address" - 2>&1); then
        join_failed=1
    fi

    if (( join_failed )); then
        join_output=${join_output//"$token"/[redacted]}
        unset token
        if [[ -n $join_output ]]; then printf '%s\n' "$join_output" >&2; fi
        fail 'lgtm-client could not join the server.'
    fi
    unset token
}

install_systemd_unit() {
    mkdir -p -- "${unit_file%/*}" 2>/dev/null ||
        fail 'The systemd user service folder could not be created.'
    tmp_unit=$(mktemp -- "${unit_file}.XXXXXX" 2>/dev/null) ||
        fail 'A temporary systemd user service file could not be created.'
    sed -e "s|^ExecStart=@LGTM_CLIENT_BIN@ run\$|ExecStart=$bin run|" \
        -e "s|^Environment=LGTM_CLIENT_STATE_DIR=@LGTM_CLIENT_STATE_DIR@\$|Environment=LGTM_CLIENT_STATE_DIR=$state_dir|" \
        -e "s|^Environment=PATH=@LGTM_CLIENT_PATH@\$|Environment=PATH=$(service_path)|" \
        "$repo/deploy/lgtm-client.service" >"$tmp_unit" 2>/dev/null &&
        replace_file "$tmp_unit" "$unit_file" 2>/dev/null ||
        fail 'The systemd user service could not be installed.'
    tmp_unit=''

    manage_unit daemon-reload
    manage_unit enable lgtm-client.service
    manage_unit restart lgtm-client.service
}

install_launch_agent() {
    local log_file=$HOME/Library/Logs/lgtm-client.log
    mkdir -p -- "${unit_file%/*}" "${log_file%/*}" 2>/dev/null ||
        fail 'The launch agent or log folder could not be created.'
    tmp_unit=$(mktemp -- "${unit_file}.XXXXXX" 2>/dev/null) ||
        fail 'A temporary launch agent file could not be created.'
    sed -e "s|<string>@LGTM_CLIENT_BIN@</string>|<string>$bin</string>|" \
        -e "s|<string>@LGTM_CLIENT_STATE_DIR@</string>|<string>$state_dir</string>|" \
        -e "s|<string>@LGTM_CLIENT_PATH@</string>|<string>$(service_path)</string>|" \
        -e "s|<string>@LGTM_CLIENT_LOG@</string>|<string>$log_file</string>|" \
        "$repo/deploy/dev.lgtmstudio.client.plist" >"$tmp_unit" 2>/dev/null &&
        replace_file "$tmp_unit" "$unit_file" 2>/dev/null ||
        fail 'The launch agent could not be installed.'
    tmp_unit=''

    bootout_launch_agent
    launchctl bootstrap "$launch_domain" "$unit_file" >/dev/null 2>&1 ||
        fail 'launchctl bootstrap dev.lgtmstudio.client failed.'
    launchctl enable "$launch_target" >/dev/null 2>&1 ||
        fail 'launchctl enable dev.lgtmstudio.client failed.'
}

enable_linger() {
    local user
    user=$(id -un 2>/dev/null) || fail 'The current user name could not be read.'
    if [[ $(loginctl show-user "$user" --property=Linger --value 2>/dev/null) == yes ]]; then
        return 0
    fi
    if ! loginctl enable-linger "$user" >/dev/null 2>&1; then
        printf 'loginctl enable-linger %s failed, so the client starts only after this user logs in.\n' "$user" >&2
        return 1
    fi
}

wait_for_service() {
    local deadline=$((SECONDS + 10))
    until [[ $(systemctl --user is-active lgtm-client.service 2>/dev/null) == active ]]; do
        (( SECONDS < deadline )) ||
            fail 'lgtm-client.service did not start; read journalctl --user -u lgtm-client.service.'
        sleep 1
    done
}

wait_for_launch_agent() {
    local deadline=$((SECONDS + 10)) agent_status
    until agent_status=$(launchctl print "$launch_target" 2>/dev/null) &&
        grep -Eq '^[[:blank:]]*state = running[[:blank:]]*$' <<< "$agent_status"; do
        (( SECONDS < deadline )) ||
            fail 'dev.lgtmstudio.client did not start; read Library/Logs/lgtm-client.log in your home folder.'
        sleep 1
    done
}

main() {
    if (( $# > 1 )) || [[ $# == 1 && $1 != --remove ]]; then
        printf '%s\n' 'Usage: deploy/install-client.sh [--remove]' >&2
        exit 2
    fi

    local repo uid os bin unit_file state_dir address tmp_bin='' tmp_unit=''
    local launch_domain launch_target
    repo=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P) ||
        fail 'The repository folder could not be found.'
    cd -- "$repo" || fail 'The repository folder could not be opened.'

    uid=$(id -u 2>/dev/null) || fail 'The current user identifier could not be read.'
    [[ $uid != 0 ]] || fail 'install-client.sh must run as the user who runs the harnesses, not as root.'
    os=$(uname -s 2>/dev/null) || fail 'The operating system could not be identified.'
    case $os in
        Linux)
            systemctl --user show-environment >/dev/null 2>&1 ||
                fail 'This Linux system has no systemd user manager for this user.'
            ;;
        Darwin)
            launch_domain=gui/$uid
            launch_target=$launch_domain/dev.lgtmstudio.client
            launchctl print "$launch_domain" >/dev/null 2>&1 ||
                fail 'macOS starts lgtm-client at login; run install-client.sh from a logged-in session.'
            ;;
        *) fail 'install-client.sh supports Linux and macOS only.' ;;
    esac
    safe_absolute_path "${HOME:-}" ||
        fail 'The home folder path holds a character that a service file cannot carry.'

    bin=$HOME/.local/bin/lgtm-client
    if [[ $os == Darwin ]]; then
        unit_file=$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist
    else
        unit_file=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/lgtm-client.service
    fi
    if [[ ${1:-} == --remove ]]; then
        if [[ $os == Darwin ]]; then
            remove_launch_agent
        else
            remove_client
        fi
        return
    fi

    local requested_state=${LGTM_CLIENT_STATE_DIR:-}
    if [[ -e $unit_file ]]; then
        if [[ $os == Darwin ]]; then
            state_dir=$(launch_agent_state_dir 2>/dev/null) ||
                fail 'The installed service could not be read.'
        else
            state_dir=$(sed -n 's/^Environment=LGTM_CLIENT_STATE_DIR=//p' "$unit_file" 2>/dev/null) ||
                fail 'The installed service could not be read.'
        fi
        if [[ -n $requested_state && $requested_state != "$state_dir" ]]; then
            fail 'LGTM_CLIENT_STATE_DIR names another folder than the installed service; run install-client.sh --remove first.'
        fi
    else
        state_dir=${requested_state:-$HOME/.local/state/lgtm-client}
    fi
    safe_absolute_path "$state_dir" || fail 'LGTM_CLIENT_STATE_DIR must be an absolute path.'

    local tool
    for tool in go docker curl jq; do
        command -v "$tool" >/dev/null 2>&1 || fail "install-client.sh needs $tool on PATH."
    done

    local published port
    published=$(compose port web 8080 && printf '.') ||
        fail 'docker compose could not read the stack; check that this user may run docker.'
    published=${published%.}
    published=${published%$'\n'}
    port=${published#127.0.0.1:}
    if [[ $published != 127.0.0.1:* || ! $port =~ ^[0-9]{1,5}$ ]] ||
        (( 10#$port < 1 || 10#$port > 65535 )); then
        fail "The stack's web service publishes no loopback port. Start it with deploy/up.sh."
    fi
    address=http://$published
    get_server || fail "The stack does not answer at $address. Start it with deploy/up.sh."

    local joined=0
    if [[ -f $state_dir/identity.json ]]; then
        joined=1
    elif [[ -e $unit_file ]]; then
        fail "The installed service's folder $state_dir holds no identity; restore it, or run install-client.sh --remove and install again."
    elif [[ -e $state_dir/machine.key ]]; then
        fail "A join into $state_dir did not finish. Remove its waiting machine on the Machines page if one is shown, move $state_dir aside, and run install-client.sh again."
    fi

    trap cleanup EXIT
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM
    mkdir -p -- "${bin%/*}" 2>/dev/null || fail 'The program folder could not be created.'
    tmp_bin=$(mktemp -- "${bin}.XXXXXX" 2>/dev/null) ||
        fail 'A temporary file for the program could not be created.'
    local version
    version=$(build_version)
    CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$version" \
        -o "$tmp_bin" ./src/client/cmd/lgtm-client >/dev/null 2>&1 &&
        version=$("$tmp_bin" version 2>/dev/null) || fail 'go build of lgtm-client failed.'

    if (( joined )); then
        printf '%s\n' 'lgtm-client is already joined.'
    else
        join_stack
    fi

    chmod 0755 -- "$tmp_bin" 2>/dev/null &&
        replace_file "$tmp_bin" "$bin" 2>/dev/null || fail 'The program could not be installed.'
    tmp_bin=''
    printf 'built lgtm-client %s\n' "$version"

    if [[ $os == Darwin ]]; then
        install_launch_agent
        wait_for_launch_agent
        printf '%s\n' 'lgtm-client runs as the launch agent dev.lgtmstudio.client; the Machines page shows whether it connects.'
        exit 0
    fi

    install_systemd_unit
    local status=0
    enable_linger || status=1
    wait_for_service
    printf '%s\n' 'lgtm-client runs as the user service lgtm-client.service; the Machines page shows whether it connects.'
    exit "$status"
}

main "$@"
