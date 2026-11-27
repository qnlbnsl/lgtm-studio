#!/usr/bin/env bash
# Hermetic tests of deploy/install-client.sh on Linux (work item 2 of the plan go-s3-stack-client).
#
# Every outside program is a stand-in: go, docker, systemctl, loginctl, launchctl, uname, id, curl and jq
# (jq passes through to the real program). Each test runs the installer with an empty environment, a home
# folder, a temporary folder and a search path that all sit inside one folder made by mktemp -d. Nothing here
# touches the real home folder, the real user service folder, a real service manager or Docker.
#
# One PASS or FAIL line is printed for each check, then one TAP line for each test, then a count.
# The exit status is nonzero when a check fails.

set -u
set -o pipefail
export LC_ALL=C

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
installer=${LGTM_INSTALLER_UNDER_TEST:-$root/deploy/install-client.sh}
template=$root/deploy/lgtm-client.service
plist_template=$root/deploy/dev.lgtmstudio.client.plist

WORK=$(mktemp -d) || exit 1
cleanup() { rm -rf -- "$WORK"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

PASSED=0
FAILCOUNT=0
TESTNO=0
TESTFAILED=0
TESTNAME=''

# An obviously false token of 60 characters that matches the shape the installer accepts.
TOKEN=lgtm1_$(printf 'NOTAREALTOKEN%.0s' 1 2 3 4 5)
TOKEN=${TOKEN:0:60}
MARKER=SHIM_SECRET_MARKER_7f3a91
ME=$(id -un)

real_bash=$(command -v bash)
real_jq=$(command -v jq)
real_id=$(command -v id)
real_uname=$(command -v uname)
for need in "$real_jq" "$real_id" "$real_uname" "$(command -v git)" "$(command -v sha256sum)"; do
    [[ -n $need ]] || { printf 'FAIL [setup] a tool the tests need is missing\n'; exit 1; }
done

# ---------------------------------------------------------------------------
# Reporting
# ---------------------------------------------------------------------------

# check <description> <command...> records one expectation of the current test.
check() {
    local description=$1
    shift
    if "$@" >/dev/null 2>&1; then
        PASSED=$((PASSED + 1))
        printf 'PASS [%s] %s\n' "$TESTNAME" "$description"
    else
        FAILCOUNT=$((FAILCOUNT + 1))
        TESTFAILED=1
        printf 'FAIL [%s] %s\n' "$TESTNAME" "$description"
    fi
}

begin() { TESTNAME=$1; TESTNO=$((TESTNO + 1)); TESTFAILED=0; }
end() {
    if ((TESTFAILED)); then
        printf 'not ok %s - %s\n' "$TESTNO" "$TESTNAME"
    else
        printf 'ok %s - %s\n' "$TESTNO" "$TESTNAME"
    fi
}

# ---------------------------------------------------------------------------
# The stand-ins. One script serves every name; it acts by the name it was called by.
# It lives outside the test folders, so that a search for the token never reads it.
# ---------------------------------------------------------------------------

SHIMDIR=$WORK/shim-source
mkdir -p "$SHIMDIR" || exit 1
cat >"$SHIMDIR/shim" <<SHIM
#!$real_bash
# Stand-in for an outside program. Written by the test writer.
name=\${0##*/}
{
    printf '%s' "\$name"
    for a in "\$@"; do printf ' %s' "\$a"; done
    printf '\n'
} >>"\$CALLS_LOG"
REAL_JQ=$real_jq
REAL_ID=$real_id
REAL_UNAME=$real_uname
REAL_BASH=$real_bash
SHIM
cat >>"$SHIMDIR/shim" <<'SHIM'
home_unit=${HOME:-/nonexistent}/.config/systemd/user/lgtm-client.service
case $name in
    jq)
        exec "$REAL_JQ" "$@"
        ;;
    id)
        if [[ ${1:-} == -u && $# -eq 1 ]]; then
            printf '%s\n' "${SHIM_UID:-1000}"
            exit 0
        fi
        exec "$REAL_ID" "$@"
        ;;
    uname)
        if [[ ${1:-} == -s ]]; then
            printf '%s\n' "${SHIM_UNAME:-Linux}"
            exit 0
        fi
        exec "$REAL_UNAME" "$@"
        ;;
    curl)
        [[ -z ${SHIM_CURL_FAIL:-} ]] || exit 7
        body=${SHIM_CURL_BODY-'{"product": "lgtm-studio", "contract": 1}'}
        out=''
        fmt=''
        while (($#)); do
            case $1 in
                -o | --output) out=$2; shift ;;
                -w | --write-out) fmt=$2; shift ;;
            esac
            shift
        done
        if [[ -n $out ]]; then
            printf '%s' "$body" >"$out"
        else
            printf '%s' "$body"
        fi
        [[ -z $fmt ]] || printf '%s' "${fmt//%\{http_code\}/200}"
        exit 0
        ;;
    docker)
        if [[ -n ${SHIM_DOCKER_FAIL:-} ]]; then
            printf '%s\n' "$SHIM_SECRET_MARKER" >&2
            exit 1
        fi
        verb=''
        for a in "$@"; do
            case $a in port | exec) verb=$a; break ;; esac
        done
        case $verb in
            port)
                if [[ -z ${SHIM_WEB_PORT+set} ]]; then
                    printf '127.0.0.1:8787\n'
                elif [[ -n $SHIM_WEB_PORT ]]; then
                    printf '127.0.0.1:%s\n' "$SHIM_WEB_PORT"
                fi
                ;;
            exec)
                if [[ -n ${SHIM_EXEC_FAIL:-} ]]; then
                    printf '%s\n' "$SHIM_SECRET_MARKER"
                    printf '%s\n' "$SHIM_SECRET_MARKER" >&2
                    exit 1
                fi
                printf '%s\n' "$SHIM_TOKEN"
                ;;
        esac
        exit 0
        ;;
    go)
        [[ -z ${SHIM_GO_FAIL:-} ]] || { printf 'go: stand-in build failed\n' >&2; exit 1; }
        out=''
        prev=''
        for a in "$@"; do
            [[ $prev == -o ]] && out=$a
            prev=$a
        done
        [[ -n $out ]] || exit 1
        {
            printf '#!%s\n' "$REAL_BASH"
            printf 'CALLS_LOG=%q\nJOIN_EXIT=%q\n' "$CALLS_LOG" "${SHIM_JOIN_EXIT:-0}"
            cat <<'CLIENT'
case ${1:-} in
    version)
        echo 0.0.0-test
        ;;
    join)
        if [[ $# -ne 3 || $3 != - ]]; then
            printf '%s\n' 'client received the token in the argument list' >>"$CALLS_LOG"
            exit 2
        fi
        if IFS= read -r token; then
            token=${token%$'\r'}
        fi
        [[ -n $token ]] || exit 2
        hash=$(printf '%s' "$token" | sha256sum | cut -d' ' -f1)
        dir=${LGTM_CLIENT_STATE_DIR:-$HOME/.local/state/lgtm-client}
        if [[ -e $HOME/.local/bin/lgtm-client || -e $HOME/.config/systemd/user/lgtm-client.service ||
            -e $HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist ]]; then
            state=present
        else
            state=absent
        fi
        printf 'client join %s %s %s installed=%s\n' "$2" "$hash" "$dir" "$state" >>"$CALLS_LOG"
        mkdir -p "$dir"
        if [[ $JOIN_EXIT != 0 ]]; then
            : >"$dir/machine.key"
            echo '{"msg":"join_refused","code":"token_used"}'
            exit "$JOIN_EXIT"
        fi
        echo 'machine-key-of-the-stand-in' >"$dir/machine.key"
        echo '{"machineId":"stand-in-machine"}' >"$dir/identity.json"
        ;;
    run)
        exit 0
        ;;
esac
CLIENT
        } >"$out"
        chmod 700 "$out"
        exit 0
        ;;
    systemctl)
        case "$*" in
            '--user show-environment')
                [[ -z ${SHIM_NO_USER_MANAGER:-} ]] || exit 1
                echo 'HOME=stand-in'
                ;;
            '--user is-active '*) echo active ;;
            '--user is-enabled '*) echo enabled ;;
            '--user disable --now '*)
                [[ -e $home_unit ]] || exit 1
                ;;
        esac
        exit 0
        ;;
    loginctl)
        case ${1:-} in
            show-user)
                if [[ -e ${SHIM_STATE:-/nonexistent}/linger ]]; then echo yes; else echo no; fi
                ;;
            enable-linger)
                [[ -z ${SHIM_LINGER_FAIL:-} ]] || exit 1
                : >"$SHIM_STATE/linger"
                ;;
        esac
        exit 0
        ;;
    launchctl)
        # Added for item 3: no login session, and a failing bootout, each steered by a variable.
        if [[ ${1:-} == print && ${2:-} =~ ^gui/[0-9]+$ ]]; then
            [[ -z ${SHIM_NO_GUI_SESSION:-} ]] || exit 113
        fi
        if [[ ${1:-} == bootout && -n ${SHIM_BOOTOUT_FAIL:-} ]]; then
            exit 1
        fi
        if [[ ${1:-} == print && ${2:-} == */dev.lgtmstudio.client ]]; then
            echo 'state = running'
        fi
        exit 0
        ;;
esac
exit 0
SHIM
chmod 755 "$SHIMDIR/shim"

tool_names=(bash env cat mktemp mv rm mkdir rmdir chmod stat sed grep awk tr head tail sort date sleep dirname
    basename readlink realpath cp ln ls sha256sum od cut wc uniq tee xargs find git printf true false test
    expr touch install diff cmp dd hostname timeout seq flock whoami logname printenv groups base64 tty
    getconf nproc nohup sync mkfifo shuf)
shim_names=(go docker systemctl loginctl launchctl uname id curl jq)

# ---------------------------------------------------------------------------
# One folder for each run of a scenario
# ---------------------------------------------------------------------------

T='' H='' LOG='' OUT='' RC=0

# fresh makes the folder of a scenario: home, tmp, shims, tools and an empty calls.log.
fresh() {
    T=$(mktemp -d "$WORK/t.XXXXXX") || return 1
    H=$T/home
    LOG=$T/calls.log
    mkdir -p "$H" "$T/tmp" "$T/shims" "$T/tools" "$T/state"
    : >"$LOG"
    local n p
    for n in "${shim_names[@]}"; do ln -s "$SHIMDIR/shim" "$T/shims/$n"; done
    for n in "${tool_names[@]}"; do
        p=$(command -v "$n" 2>/dev/null) || continue
        [[ $p == /* ]] && ln -s "$p" "$T/tools/$n"
    done
    return 0
}

# run [NAME=value ...] [-- argument ...] runs the installer. OUT holds both output streams, RC the status.
run() {
    local -a envs=() args=()
    while (($#)); do
        if [[ $1 == -- ]]; then
            shift
            args=("$@")
            break
        fi
        envs+=("$1")
        shift
    done
    OUT=$(
        cd "$T" &&
            umask 077 &&
            env -i HOME="$H" PATH="$T/shims:$T/tools" TMPDIR="$T/tmp" USER="$ME" LOGNAME="$ME" LANG=C \
                GIT_OPTIONAL_LOCKS=0 CALLS_LOG="$LOG" SHIM_STATE="$T/state" SHIM_TOKEN="$TOKEN" \
                SHIM_SECRET_MARKER="$MARKER" "${envs[@]}" "$installer" "${args[@]}" 2>&1 </dev/null
    )
    RC=$?
}

# runx is run, but through bash -x.
runx() {
    OUT=$(
        cd "$T" &&
            umask 077 &&
            env -i HOME="$H" PATH="$T/shims:$T/tools" TMPDIR="$T/tmp" USER="$ME" LOGNAME="$ME" LANG=C \
                GIT_OPTIONAL_LOCKS=0 CALLS_LOG="$LOG" SHIM_STATE="$T/state" SHIM_TOKEN="$TOKEN" \
                SHIM_SECRET_MARKER="$MARKER" "$@" "$real_bash" -x "$installer" 2>&1 </dev/null
    )
    RC=$?
}

bin() { printf '%s' "$H/.local/bin/lgtm-client"; }
unit() { printf '%s' "$H/.config/systemd/user/lgtm-client.service"; }
agent() { printf '%s' "$H/Library/LaunchAgents/dev.lgtmstudio.client.plist"; }
default_state() { printf '%s' "$H/.local/state/lgtm-client"; }

# Lines of the calls log, and the lines that a run added after a mark.
mark() { wc -l <"$LOG"; }
since() { tail -n +"$(($1 + 1))" "$LOG"; }
has_line() { grep -qxF -- "$1" "$LOG"; }
has_re() { grep -qE -- "$1" "$LOG"; }
no_re() { ! grep -qE -- "$1" "$LOG"; }
# line_of <exact line> prints the number of its first occurrence.
line_of() { grep -nxF -m1 -- "$1" "$LOG" | cut -d: -f1; }
line_of_re() { grep -nE -m1 -- "$1" "$LOG" | cut -d: -f1; }
count_of() { grep -cxF -- "$1" "$LOG"; }
count_re() { grep -cE -- "$1" "$LOG"; }

out_has() { [[ $OUT == *"$1"* ]]; }
out_lacks() { [[ $OUT != *"$1"* ]]; }
rc_is() { ((RC == $1)); }

nothing_installed() { [[ ! -e $(bin) && ! -e $(unit) && ! -e $(agent) ]]; }
bindir_empty() { [[ ! -d $H/.local/bin ]] || [[ -z $(find "$H/.local/bin" -mindepth 1 -print -quit) ]]; }

# The lines about a service manager that a refused run may hold.
manager_lines_beyond_probe() {
    grep -E '^(systemctl|loginctl|launchctl) ' <<<"$1" |
        grep -vxE 'systemctl --user show-environment|launchctl print .*' || true
}

# snap prints a digest of the files and the modes under the given paths.
snap() {
    local p
    for p in "$@"; do
        [[ -e $p ]] || { printf 'absent %s\n' "$p"; continue; }
        find "$p" -printf '%p %y %m\n' | sort
        find "$p" -type f -exec sha256sum {} + | sort
    done
}

# ---------------------------------------------------------------------------
# The tests
# ---------------------------------------------------------------------------

unit_text_equals_template() { # <state folder> <path value> <unit file>
    local t expected
    t=$(cat "$template") || return 1
    t=${t//@LGTM_CLIENT_BIN@/$(bin)}
    t=${t//@LGTM_CLIENT_STATE_DIR@/$1}
    t=${t//@LGTM_CLIENT_PATH@/$2}
    expected=$T/expected.unit
    printf '%s\n' "$t" >"$expected"
    cmp -s "$expected" "$3"
}

mode_is() { [[ $(stat -c %a -- "$1" 2>/dev/null) == "$2" ]]; }

test_installs_linux_user_service() {
    begin installs_linux_user_service
    fresh
    run SHIM_UNAME=Linux
    check 'the exit code is 0' rc_is 0
    check 'the program exists' test -f "$(bin)"
    check 'the program has mode 0755' mode_is "$(bin)" 755
    check 'the unit file equals the template with the placeholders replaced' \
        unit_text_equals_template "$(default_state)" "$T/shims:$T/tools" "$(unit)"
    check 'the unit holds Restart=on-failure' grep -qxF 'Restart=on-failure' "$(unit)"
    check 'the unit holds RestartSec=5' grep -qxF 'RestartSec=5' "$(unit)"
    check 'the unit holds StartLimitIntervalSec=0' grep -qxF 'StartLimitIntervalSec=0' "$(unit)"
    local a b c
    a=$(line_of 'systemctl --user daemon-reload')
    b=$(line_of 'systemctl --user enable lgtm-client.service')
    c=$(line_of 'systemctl --user restart lgtm-client.service')
    check 'daemon-reload, enable and restart appear in that order' \
        test -n "$a" -a -n "$b" -a -n "$c" -a "${a:-0}" -lt "${b:-0}" -a "${b:-0}" -lt "${c:-0}"
    check 'lingering is turned on for the user' has_line "loginctl enable-linger $ME"
    check 'go builds with the version flag' has_re '^go build .*-X main\.version=0\.0\.0-'
    check 'go builds the client command' has_re '^go build .*\./src/client/cmd/lgtm-client'
    check 'the output says what was built' out_has 'built lgtm-client 0.0.0-'
    check 'the output ends with the service sentence' out_has 'lgtm-client runs as the user service lgtm-client.service'
    end
}

test_asks_server_for_hosts_server_token() {
    begin asks_server_for_hosts_server_token
    local compose_file=$root/deploy/compose.yaml phys
    phys=$(cd "$root" && pwd -P)/deploy/compose.yaml
    local port_line exec_line a b

    fresh
    run SHIM_UNAME=Linux
    check 'the run succeeds' rc_is 0
    a=$(line_of_re "^docker compose -f ($compose_file|$phys) port web 8080\$")
    b=$(line_of_re "^docker compose -f ($compose_file|$phys) exec -T server lgtm-server join-token --hosts-server\$")
    check 'port is asked first with the repository compose file' test -n "$a"
    check 'the token is asked after the port' test -n "$b" -a "${a:-0}" -lt "${b:-0}"
    check 'the token is asked exactly once' test "$(count_re "^docker compose -f ($compose_file|$phys) exec ")" -eq 1
    check 'the join line holds the default address' has_re '^client join http://127\.0\.0\.1:8787 '

    fresh
    run SHIM_UNAME=Linux COMPOSE_FILE="$T/other.yaml"
    check 'with COMPOSE_FILE set the run succeeds' rc_is 0
    check 'with COMPOSE_FILE set port has no -f' has_line 'docker compose port web 8080'
    check 'with COMPOSE_FILE set the token call has no -f' \
        has_line 'docker compose exec -T server lgtm-server join-token --hosts-server'

    fresh
    run SHIM_UNAME=Linux COMPOSE_FILE=
    check 'with COMPOSE_FILE empty the run succeeds' rc_is 0
    check 'with COMPOSE_FILE empty the launcher rule uses -f' has_re "^docker compose -f ($compose_file|$phys) port web 8080\$"

    fresh
    run SHIM_UNAME=Linux SHIM_WEB_PORT=8899
    check 'with another port the join line holds that address' has_re '^client join http://127\.0\.0\.1:8899 '

    fresh
    run SHIM_UNAME=Linux SHIM_WEB_PORT=
    check 'with no published port the run exits 1' rc_is 1
    check 'with no published port the output says so' out_has 'publishes no loopback port'
    check 'with no published port nothing is built' no_re '^go '
    check 'with no published port nothing is joined' no_re '^client join '
    port_line=''
    end
}

field() { # <line> <number>
    awk -v n="$2" '{print $n}' <<<"$1"
}

test_joins_before_service_starts() {
    begin joins_before_service_starts
    fresh
    run SHIM_UNAME=Linux
    local jl jn first_sys unit_dir sums
    jl=$(grep -E '^client join ' "$LOG" | head -n1)
    jn=$(line_of_re '^client join ')
    first_sys=$(line_of_re '^systemctl --user (daemon-reload|enable|restart|start) |^systemctl --user daemon-reload$|lgtm-client\.service')
    check 'the run succeeds and joins' test -n "$jl"
    check 'the join line ends with installed=absent' test "${jl##* }" = installed=absent
    check 'the join comes before the first service-manager line' test -n "$jn" -a -n "$first_sys" -a "${jn:-0}" -lt "${first_sys:-0}"
    check 'the join hash equals the SHA-256 of the token' \
        test "$(field "$jl" 4)" = "$(printf '%s' "$TOKEN" | sha256sum | cut -d' ' -f1)"
    unit_dir=$(sed -n 's/^Environment=LGTM_CLIENT_STATE_DIR=//p' "$(unit)" 2>/dev/null)
    check 'the join folder equals the folder in the unit' test -n "$unit_dir" -a "$(field "$jl" 5)" = "$unit_dir"

    fresh
    mkdir -p "$T/custom"
    run SHIM_UNAME=Linux LGTM_CLIENT_STATE_DIR="$T/custom"
    jl=$(grep -E '^client join ' "$LOG" | head -n1)
    check 'with the variable set the join names that folder' test "$(field "$jl" 5)" = "$T/custom"
    check 'with the variable set the unit names that folder' grep -qxF "Environment=LGTM_CLIENT_STATE_DIR=$T/custom" "$(unit)"

    fresh
    run SHIM_UNAME=Linux LGTM_CLIENT_STATE_DIR=relative/dir
    check 'a relative folder exits 1' rc_is 1
    check 'a relative folder says the path must be absolute' out_has 'must be an absolute path'
    check 'a relative folder builds nothing' no_re '^go '
    sums=''
    end
}

test_token_never_printed() {
    begin token_never_printed
    local hash
    hash=$(printf '%s' "$TOKEN" | sha256sum | cut -d' ' -f1)
    fresh
    run SHIM_UNAME=Linux
    if has_line 'client received the token in the argument list'; then
        check 'The token was in the argument list.' false
    else
        check 'the token stays out of the argument list' true
    fi
    check 'the join line with the token hash is in the calls log' has_re "^client join [^ ]+ $hash "
    check 'the output does not hold the token' out_lacks "$TOKEN"
    check 'no file of the test folder holds the token' \
        bash -c '! grep -rIlF -- "$1" "$2" 2>/dev/null | grep -q .' _ "$TOKEN" "$T"

    fresh
    run SHIM_UNAME=Linux SHIM_JOIN_EXIT=1
    check 'a failed join reached the join' has_re "^client join [^ ]+ $hash "
    check 'a failed join does not print the token' out_lacks "$TOKEN"
    check 'no file of the failed test folder holds the token' \
        bash -c '! grep -rIlF -- "$1" "$2" 2>/dev/null | grep -q .' _ "$TOKEN" "$T"

    fresh
    runx SHIM_UNAME=Linux
    check 'a trace run reached the join' has_re "^client join [^ ]+ $hash "
    check 'the trace does not hold the token' out_lacks "$TOKEN"
    end
}

test_join_failure_installs_no_service() {
    begin join_failure_installs_no_service
    fresh
    run SHIM_UNAME=Linux SHIM_JOIN_EXIT=1
    check 'the join was attempted' has_re '^client join '
    check 'the exit code is 1' rc_is 1
    check 'the output holds the join refusal' out_has 'join_refused'
    check 'the output says the client could not join' out_has 'lgtm-client could not join the server.'
    check 'neither the program nor the unit exists' nothing_installed
    check 'no temporary file is left in the program folder' bindir_empty
    check 'no service was enabled' no_re '^systemctl --user enable'
    run SHIM_UNAME=Linux SHIM_JOIN_EXIT=1
    check 'a second run exits 1' rc_is 1
    check 'a second run says the join did not finish' out_has 'did not finish'
    end
}

test_refuses_root() {
    begin refuses_root
    fresh
    run SHIM_UNAME=Linux SHIM_UID=0
    check 'the exit code is 1' rc_is 1
    check 'the output says not as root' out_has 'not as root'
    check 'nothing is built' no_re '^go '
    check 'nothing is installed' nothing_installed
    end
}

test_refuses_missing_tool() {
    begin refuses_missing_tool
    local tool
    for tool in go docker curl jq; do
        fresh
        rm -f -- "$T/shims/$tool"
        run SHIM_UNAME=Linux
        check "without $tool the exit code is 1" rc_is 1
        check "without $tool the output says it needs $tool" out_has "needs $tool"
        check "without $tool nothing is joined" no_re '^client join '
        check "without $tool nothing is installed" nothing_installed
    done
    end
}

test_refuses_docker_failure() {
    begin refuses_docker_failure
    fresh
    run SHIM_UNAME=Linux SHIM_DOCKER_FAIL=1
    check 'the exit code is 1' rc_is 1
    check 'the output says who may run docker' out_has 'may run docker'
    check 'the output does not hold the secret marker' out_lacks "$MARKER"
    check 'nothing is built' no_re '^go '

    fresh
    run SHIM_UNAME=Linux SHIM_EXEC_FAIL=1
    check 'a failed token command exits 1' rc_is 1
    check 'a failed token command says it returned no token' out_has 'returned no token'
    check 'a failed token command does not print the marker' out_lacks "$MARKER"
    check 'a failed token command joins nothing' no_re '^client join '
    check 'a failed token command installs nothing' nothing_installed
    end
}

test_refuses_stack_not_answering() {
    begin refuses_stack_not_answering
    local case_env
    local -a cases=(
        'SHIM_CURL_FAIL=1'
        'SHIM_CURL_BODY={"product": "lgtm-studio", "contract": 1, "sample": true}'
        'SHIM_CURL_BODY={"product": "other", "contract": 1}'
        'SHIM_CURL_BODY={"product": "lgtm-studio", "contract": 0}'
        'SHIM_CURL_BODY={"product": "lgtm-studio"}'
        'SHIM_CURL_BODY=not json'
    )
    for case_env in "${cases[@]}"; do
        fresh
        run SHIM_UNAME=Linux "$case_env"
        check "[$case_env] the exit code is 1" rc_is 1
        check "[$case_env] the output points to deploy/up.sh" out_has 'deploy/up.sh'
        check "[$case_env] no token was asked for" no_re ' exec '
        check "[$case_env] nothing was built" no_re '^go '
    done
    end
}

test_refuses_malformed_token() {
    begin refuses_malformed_token
    local bad
    for bad in oops 'lgtm1_abc def' 'lgtm1_' 'xlgtm1_abcdef'; do
        fresh
        run SHIM_UNAME=Linux SHIM_TOKEN="$bad"
        check "[$bad] the exit code is 1" rc_is 1
        check "[$bad] the output says no token was returned" out_has 'returned no token'
        check "[$bad] nothing is joined" no_re '^client join '
        check "[$bad] nothing is installed" nothing_installed
    done
    end
}

test_refuses_missing_user_manager() {
    begin refuses_missing_user_manager
    fresh
    run SHIM_UNAME=Linux SHIM_NO_USER_MANAGER=1
    check 'the exit code is 1' rc_is 1
    check 'the output names the missing user manager' out_has 'no systemd user manager'
    check 'nothing is built' no_re '^go '
    end
}

test_refuses_unsafe_home() {
    begin refuses_unsafe_home
    local bad
    fresh
    mkdir -p "$T/ho me"
    for bad in "$T/ho me" "$T/ho%me" "$T/ho\$me" 'relative/home' ''; do
        run SHIM_UNAME=Linux HOME="$bad"
        check "[$bad] the exit code is 1" rc_is 1
        check "[$bad] the output says the path holds a character a service file cannot carry" \
            out_has 'cannot carry'
        check "[$bad] nothing is built" no_re '^go '
        check "[$bad] nothing is joined" no_re '^client join '
    done
    end
}

test_refuses_unfinished_join() {
    begin refuses_unfinished_join
    fresh
    mkdir -p "$(default_state)"
    : >"$(default_state)/machine.key"
    run SHIM_UNAME=Linux
    check 'the exit code is 1' rc_is 1
    check 'the output says the join did not finish' out_has 'did not finish'
    check 'no token was asked for' no_re ' exec '
    check 'nothing is installed' nothing_installed

    fresh
    mkdir -p "$T/custom"
    : >"$T/custom/machine.key"
    run SHIM_UNAME=Linux LGTM_CLIENT_STATE_DIR="$T/custom"
    check 'with a set folder the exit code is 1' rc_is 1
    check 'with a set folder the output says the join did not finish' out_has 'did not finish'
    check 'with a set folder no token was asked for' no_re ' exec '
    end
}

test_build_failure_stops_installer() {
    begin build_failure_stops_installer
    fresh
    run SHIM_UNAME=Linux SHIM_GO_FAIL=1
    check 'the exit code is 1' rc_is 1
    check 'the output says the go build failed' out_has 'go build'
    check 'a build was attempted' has_re '^go build '
    check 'no token was asked for' no_re ' exec '
    check 'the program folder holds no file' bindir_empty
    check 'no unit exists' test ! -e "$(unit)"
    end
}

test_unsafe_path_entries_dropped() {
    begin unsafe_path_entries_dropped
    fresh
    mkdir -p "$T/extra" "$T/a%b" "$T/sp ace" "$T/d\$ollar" "$T/q\"uote"
    local path="$T/shims:$T/a%b:$T/tools:$T/sp ace:$T/extra:$T/d\$ollar:$T/q\"uote"
    run SHIM_UNAME=Linux PATH="$path"
    check 'the run succeeds' rc_is 0
    check 'the unit path keeps the safe entries in order and drops the others' \
        grep -qxF "Environment=PATH=$T/shims:$T/tools:$T/extra" "$(unit)"
    end
}

test_linger_failure_is_reported() {
    begin linger_failure_is_reported
    fresh
    run SHIM_UNAME=Linux SHIM_LINGER_FAIL=1
    check 'the unit file exists' test -f "$(unit)"
    check 'the service was restarted' has_line 'systemctl --user restart lgtm-client.service'
    check 'the output names loginctl enable-linger' out_has 'loginctl enable-linger'
    check 'the exit code is 1' rc_is 1
    end
}

test_rerun_keeps_joined_identity() {
    begin rerun_keeps_joined_identity
    fresh
    run SHIM_UNAME=Linux
    check 'the first run succeeds' rc_is 0
    local st
    st=$(default_state)
    cp -- "$st/identity.json" "$T/identity.before" 2>/dev/null
    cp -- "$st/machine.key" "$T/machine.before" 2>/dev/null
    local m
    m=$(mark)
    run SHIM_UNAME=Linux
    check 'the second run exits 0' rc_is 0
    check 'the second run asked for the port' test -n "$(since "$m" | grep -E '^docker compose .*port web 8080$')"
    check 'the second run asked for no token' test -z "$(since "$m" | grep -E ' exec ')"
    check 'the second run joined nothing' test -z "$(since "$m" | grep -E '^client join ')"
    check 'the second run reloaded the manager' test -n "$(since "$m" | grep -xF 'systemctl --user daemon-reload')"
    check 'the second run restarted the service' test -n "$(since "$m" | grep -xF 'systemctl --user restart lgtm-client.service')"
    check 'the second run built the program again' test -n "$(since "$m" | grep -E '^go build ')"
    check 'the output says the client is already joined' out_has 'lgtm-client is already joined.'
    check 'identity.json is unchanged' cmp -s "$T/identity.before" "$st/identity.json"
    check 'machine.key is unchanged' cmp -s "$T/machine.before" "$st/machine.key"
    end
}

# rerun_scenario <Linux|Darwin> runs the five-run sequence of the plan.
rerun_scenario() {
    local system=$1 s1 s2 m before after file
    fresh
    s1=$T/s1
    s2=$T/s2
    if [[ $system == Linux ]]; then file=$(unit); else file=$(agent); fi

    run SHIM_UNAME="$system" LGTM_CLIENT_STATE_DIR="$s1"
    check "[$system] the first run with a folder exits 0" rc_is 0
    check "[$system] the installed service names that folder" grep -qF -- "$s1" "$file"

    m=$(mark)
    run SHIM_UNAME="$system"
    check "[$system] a second run without the variable exits 0" rc_is 0
    check "[$system] the second run joins nothing" test -z "$(since "$m" | grep -E '^client join |exec ')"
    check "[$system] the second run keeps the folder in the service" grep -qF -- "$s1" "$file"
    check "[$system] the second run makes no default folder" test ! -e "$(default_state)"

    m=$(mark)
    run SHIM_UNAME="$system" LGTM_CLIENT_STATE_DIR="$s1"
    check "[$system] a third run with the same folder exits 0" rc_is 0
    check "[$system] the third run joins nothing" test -z "$(since "$m" | grep -E '^client join |exec ')"

    # A fourth run with another folder: first an empty one, then a joined one.
    local other
    for other in empty joined; do
        if [[ $other == joined ]]; then
            mkdir -p "$s2"
            echo key >"$s2/machine.key"
            echo '{"machineId":"other"}' >"$s2/identity.json"
        fi
        before=$(snap "$(bin)" "$file" "$s1" "$s2")
        m=$(mark)
        run SHIM_UNAME="$system" LGTM_CLIENT_STATE_DIR="$s2"
        after=$(snap "$(bin)" "$file" "$s1" "$s2")
        check "[$system] a run with another $other folder exits 1" rc_is 1
        check "[$system] a run with another $other folder says to remove first" out_has '--remove first'
        check "[$system] a run with another $other folder builds, asks and joins nothing" \
            test -z "$(since "$m" | grep -E '^go |exec |^client join ')"
        check "[$system] a run with another $other folder runs only the probe of the manager" \
            test -z "$(manager_lines_beyond_probe "$(since "$m")")"
        check "[$system] a run with another $other folder leaves everything unchanged" test "$before" = "$after"
    done

    # A fifth run without the variable, with the service's folder moved away.
    mv -- "$s1" "$T/s1.moved"
    before=$(snap "$(bin)" "$file" "$s1" "$s2")
    m=$(mark)
    run SHIM_UNAME="$system"
    after=$(snap "$(bin)" "$file" "$s1" "$s2")
    check "[$system] a run with the service's folder moved exits 1" rc_is 1
    check "[$system] a run with the service's folder moved says it holds no identity" out_has 'holds no identity'
    check "[$system] a run with the service's folder moved builds, asks and joins nothing" \
        test -z "$(since "$m" | grep -E '^go |exec |^client join ')"
    check "[$system] a run with the service's folder moved runs only the probe of the manager" \
        test -z "$(manager_lines_beyond_probe "$(since "$m")")"
    check "[$system] a run with the service's folder moved leaves everything unchanged" test "$before" = "$after"
}

test_rerun_uses_state_folder_of_installed_unit() {
    begin rerun_uses_state_folder_of_installed_unit
    rerun_scenario Linux
    if [[ -f $plist_template ]]; then
        rerun_scenario Darwin
    else
        printf 'SKIP [%s] the Darwin part waits for deploy/dev.lgtmstudio.client.plist of item 3\n' "$TESTNAME"
    fi
    end
}

test_remove_stops_and_deletes_service() {
    begin remove_stops_and_deletes_service
    fresh
    run SHIM_UNAME=Linux
    check 'the first run succeeds' rc_is 0
    local st before after
    st=$(default_state)
    before=$(snap "$st")
    run SHIM_UNAME=Linux LGTM_CLIENT_STATE_DIR="$T/elsewhere" -- --remove
    after=$(snap "$st")
    check '--remove exits 0 and ignores the variable' rc_is 0
    check '--remove disables and stops the service' has_line 'systemctl --user disable --now lgtm-client.service'
    check 'the unit file is gone' test ! -e "$(unit)"
    check 'the program is gone' test ! -e "$(bin)"
    check 'the manager is reloaded after the removal' test "$(count_of 'systemctl --user daemon-reload')" -ge 2
    check 'the state folder and its files are unchanged' test "$before" = "$after"
    check 'the state folder still holds the identity' test -f "$st/identity.json"
    run SHIM_UNAME=Linux -- --remove
    check 'a second removal with nothing installed exits 0' rc_is 0
    end
}

test_remove_needs_no_build_tools() {
    begin remove_needs_no_build_tools
    fresh
    run SHIM_UNAME=Linux
    check 'the first run succeeds' rc_is 0
    check 'the first run installed the unit and the program' test -f "$(unit)" -a -f "$(bin)"
    local tool
    for tool in go docker curl jq; do rm -f -- "$T/shims/$tool"; done
    run SHIM_UNAME=Linux -- --remove
    check '--remove without build tools exits 0' rc_is 0
    check 'the unit file is gone' test ! -e "$(unit)"
    check 'the program is gone' test ! -e "$(bin)"
    end
}

test_usage_on_unknown_argument() {
    begin usage_on_unknown_argument
    local arg
    for arg in --frobnicate --rejoin extra; do
        fresh
        run SHIM_UNAME=Linux -- "$arg"
        check "[$arg] the exit code is 2" rc_is 2
        check "[$arg] the calls log is empty" test ! -s "$LOG"
    done
    fresh
    run SHIM_UNAME=Linux -- --remove --remove
    check '[--remove --remove] the exit code is 2' rc_is 2
    check '[--remove --remove] the calls log is empty' test ! -s "$LOG"
    end
}

# ---------------------------------------------------------------------------
# Work item 3: the installer on macOS. These tests run with SHIM_UNAME=Darwin and SHIM_UID=501,
# so an installer that takes the user identifier from anywhere but `id -u` fails them.
# ---------------------------------------------------------------------------

LABEL=dev.lgtmstudio.client
MAC_UID=501

agent_text_equals_template() { # <state folder> <path value> <agent file>
    local t expected
    t=$(cat "$plist_template") || return 1
    t=${t//@LGTM_CLIENT_BIN@/$(bin)}
    t=${t//@LGTM_CLIENT_STATE_DIR@/$1}
    t=${t//@LGTM_CLIENT_PATH@/$2}
    t=${t//@LGTM_CLIENT_LOG@/$H/Library/Logs/lgtm-client.log}
    expected=$T/expected.plist
    printf '%s\n' "$t" >"$expected"
    cmp -s "$expected" "$3"
}

no_manager_lines() { ! grep -qE '^(systemctl|loginctl) ' "$LOG"; }

test_installs_macos_launch_agent() {
    begin installs_macos_launch_agent
    local first_launchctl j p bo bs en pr
    local gui=gui/$MAC_UID
    fresh
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID
    check 'the exit code is 0' rc_is 0
    check 'the agent file exists' test -f "$(agent)"
    check 'the agent file equals the template with its four placeholders replaced' \
        agent_text_equals_template "$(default_state)" "$T/shims:$T/tools" "$(agent)"
    check 'the program exists with mode 0755' mode_is "$(bin)" 755
    check 'no template placeholder is left in the agent file' bash -c '! grep -q "@LGTM_" "$1"' _ "$(agent)"
    check 'the agent folder holds the agent file and nothing else' \
        test "$(find "$H/Library/LaunchAgents" -type f 2>/dev/null | wc -l)" -eq 1
    first_launchctl=$(grep -m1 -E '^launchctl ' "$LOG")
    check "the first launchctl call is print $gui" test "$first_launchctl" = "launchctl print $gui"
    j=$(line_of_re '^client join .* installed=absent$')
    p=$(line_of "launchctl print $gui")
    bo=$(line_of "launchctl bootout $gui/$LABEL")
    bs=$(line_of "launchctl bootstrap $gui $(agent)")
    en=$(line_of "launchctl enable $gui/$LABEL")
    pr=$(grep -nxF -- "launchctl print $gui/$LABEL" "$LOG" | tail -n1 | cut -d: -f1)
    check 'the join line ends with installed=absent' test -n "$j"
    check 'print, then the join, then bootout, bootstrap and enable run in that order' \
        test -n "$p" -a -n "$bo" -a -n "$bs" -a -n "$en" -a -n "$j" \
        -a "${p:-0}" -lt "${j:-0}" -a "${j:-0}" -lt "${bo:-0}" -a "${bo:-0}" -lt "${bs:-0}" -a "${bs:-0}" -lt "${en:-0}"
    check 'the wait reads the agent state after the bootstrap' test -n "$pr" -a "${bs:-0}" -lt "${pr:-0}"
    check 'no systemctl and no loginctl line exists' no_manager_lines
    check 'the output says what was built' out_has 'built lgtm-client 0.0.0-'
    check 'the output does not hold the token' out_lacks "$TOKEN"
    check 'no file of the test folder holds the token' \
        bash -c '! grep -rIlF -- "$1" "$2" 2>/dev/null | grep -q .' _ "$TOKEN" "$T"

    fresh
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID SHIM_JOIN_EXIT=1
    check 'with a refused join the join was attempted' has_re '^client join '
    check 'with a refused join the exit code is 1' rc_is 1
    check 'with a refused join no agent file exists' test ! -e "$(agent)"
    check 'with a refused join the program does not exist' test ! -e "$(bin)"
    check 'with a refused join nothing was bootstrapped' no_re '^launchctl (bootstrap|enable) '

    fresh
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID SHIM_NO_GUI_SESSION=1
    check 'without a login session the exit code is 1' rc_is 1
    check 'without a login session the output says macOS starts the client at login' \
        out_has 'macOS starts lgtm-client at login; run install-client.sh from a logged-in session.'
    check 'without a login session the probe was made' has_line "launchctl print $gui"
    check 'without a login session nothing is built' no_re '^go '
    check 'without a login session nothing is joined or asked for' no_re '^client join | exec '
    check 'without a login session nothing is installed' nothing_installed
    check 'without a login session nothing is bootstrapped' no_re '^launchctl (bootout|bootstrap|enable) '
    end
}

# plist_checks <agent file> <state folder> <path value> <log file> checks the parsed agent.
plist_checks() {
    local file=$1
    if command -v plutil >/dev/null 2>&1; then
        plutil -lint "$file" >/dev/null 2>&1 || return 1
    fi
    command -v python3 >/dev/null 2>&1 || return 0
    python3 -I - "$file" "$(bin)" "$2" "$3" "$4" <<'PY'
import plistlib, sys
path, binary, state, search, log = sys.argv[1:6]
with open(path, "rb") as handle:
    d = plistlib.load(handle)
keep = d.get("KeepAlive")
env = d.get("EnvironmentVariables")
checks = [
    d.get("Label") == "dev.lgtmstudio.client",
    d.get("ProgramArguments") == [binary, "run"],
    isinstance(env, dict) and env.get("LGTM_CLIENT_STATE_DIR") == state and env.get("PATH") == search,
    d.get("RunAtLoad") is True,
    isinstance(keep, dict) and keep.get("SuccessfulExit") is False and len(keep) == 1,
    d.get("ThrottleInterval") == 5,
    d.get("StandardOutPath") == log,
    d.get("StandardErrorPath") == log,
    d.get("ProcessType") == "Background",
]
sys.exit(0 if all(checks) else 1)
PY
}

test_macos_agent_parses() {
    begin macos_agent_parses
    fresh
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID
    check 'the run reached the install' rc_is 0
    check 'the agent file exists' test -f "$(agent)"
    if ! command -v plutil >/dev/null 2>&1 && ! command -v python3 >/dev/null 2>&1; then
        printf 'ok %s - macos_agent_parses # SKIP no parser\n' "$TESTNO"
        return 0
    fi
    check 'the rendered agent file parses with the expected keys' \
        plist_checks "$(agent)" "$(default_state)" "$T/shims:$T/tools" "$H/Library/Logs/lgtm-client.log"
    end
}

test_macos_remove_boots_out_agent() {
    begin macos_remove_boots_out_agent
    local gui=gui/$MAC_UID st before after m tool
    fresh
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID
    check 'the first run succeeds and installs the agent' test "$RC" -eq 0 -a -f "$(agent)" -a -f "$(bin)"
    st=$(default_state)
    before=$(snap "$st")
    m=$(mark)
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID LGTM_CLIENT_STATE_DIR="$T/elsewhere" -- --remove
    after=$(snap "$st")
    check '--remove exits 0 and ignores the variable' rc_is 0
    check 'launchctl bootout names the agent' test -n "$(since "$m" | grep -xF "launchctl bootout $gui/$LABEL")"
    check 'the agent file is gone' test ! -e "$(agent)"
    check 'the program is gone' test ! -e "$(bin)"
    check 'the state folder and its files are unchanged' test "$before" = "$after"
    check 'no systemctl and no loginctl line exists' no_manager_lines
    check 'a removal builds and joins nothing' test -z "$(since "$m" | grep -E '^go |exec |^client join ')"

    fresh
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID
    for tool in go docker curl jq; do rm -f -- "$T/shims/$tool"; done
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID -- --remove
    check '--remove without build tools exits 0' rc_is 0
    check '--remove without build tools removes the agent file and the program' \
        test ! -e "$(agent)" -a ! -e "$(bin)"

    fresh
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID
    run SHIM_UNAME=Darwin SHIM_UID=$MAC_UID SHIM_BOOTOUT_FAIL=1 -- --remove
    check 'a failed bootout of a loaded agent exits 1' rc_is 1
    end
}

test_refuses_unsupported_system() {
    begin refuses_unsupported_system
    local system
    for system in FreeBSD MINGW64_NT-10.0 DarwinX; do
        fresh
        run SHIM_UNAME="$system"
        check "[$system] the exit code is 1" rc_is 1
        check "[$system] the output says Linux and macOS only" out_has 'Linux and macOS only'
        check "[$system] nothing is built" no_re '^go '
        check "[$system] no service manager is called" no_re '^(systemctl|loginctl|launchctl) '
        check "[$system] nothing is installed" nothing_installed
    done
    end
}

# ---------------------------------------------------------------------------

printf '1..25\n'
test_installs_linux_user_service
test_asks_server_for_hosts_server_token
test_joins_before_service_starts
test_token_never_printed
test_join_failure_installs_no_service
test_refuses_root
test_refuses_missing_tool
test_refuses_docker_failure
test_refuses_stack_not_answering
test_refuses_malformed_token
test_refuses_missing_user_manager
test_refuses_unsafe_home
test_refuses_unfinished_join
test_build_failure_stops_installer
test_unsafe_path_entries_dropped
test_linger_failure_is_reported
test_rerun_keeps_joined_identity
test_rerun_uses_state_folder_of_installed_unit
test_remove_stops_and_deletes_service
test_remove_needs_no_build_tools
test_usage_on_unknown_argument
test_installs_macos_launch_agent
test_macos_agent_parses
test_macos_remove_boots_out_agent
test_refuses_unsupported_system

printf '%s passed, %s failed\n' "$PASSED" "$FAILCOUNT"
((FAILCOUNT == 0))
