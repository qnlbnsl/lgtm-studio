#!/usr/bin/env bash
set +x
set +a
set -uo pipefail
umask 077
export LC_ALL=C

# The offline assertions do not run the installer, Docker or a service manager.
# The live observer must keep raw logs and credentials in its owned fixture.

native_fixture_preflight() {
    # This is a test-writer stub. Replace it with the read-only refusal checks.
    # The arguments are the manager entry state, binary, unit, log and store paths.
    return 0
}

native_supervisor_observe() {
    # This is a test-writer stub. Replace it with the owned native fixture and observations.
    # The arguments are the manager, case and candidate commit. An empty commit is the offline lane.
    native_observation='{}'
}

native_reject() {
    reason=$1
    return 1
}

native_check() {
    local message=$1 filter=$2
    jq -es "length == 1 and (.[0] | $filter)" <<<"$native_observation" >/dev/null 2>&1 ||
        native_reject "$message"
}

native_fixture_owns_stack() {
    native_check 'The native stack did not have a fresh project, candidate tag and private state.' '
        .fixture as $f
        | ($f.project | test("^lgtm-s4-[a-z0-9_-]+$")) and $f.image_tag == ($f.project + "-4")
        and ($f.state | startswith("/")) and ($f.checkout | startswith("/"))
        and ($f.ownership_record | type == "string" and length > 0)
        and $f.preexisting_resources == 0' &&
    native_check 'The installer selection did not resolve to the owned listener on the same Docker daemon.' '
        .fixture as $f
        | ($f.port | type == "number" and . > 0 and . <= 65535 and . != 8787 and floor == .)
        and $f.binding == ("127.0.0.1:" + ($f.port | tostring))
        and ($f.daemon | type == "string" and length > 0) and $f.installer_daemon == $f.daemon
        and ($f.web_container | type == "string" and length > 0) and $f.bound_container == $f.web_container
        and $f.installer_project == $f.project and $f.installer_binding == $f.binding'
}

requested_restart_restarts_service() {
    native_check 'The old process did not exit 75 after the restart request.' '
        .reconnect_status == 200 and .process.exit_status == 75
        and (.process.old_pid | type == "number" and . > 0)
        and .process.exit_pid == .process.old_pid' &&
    native_check 'The manager did not start one replacement on the same approved enrollment.' '
        (.process.replacement_pid | type == "number" and . > 0)
        and .process.replacement_pid != .process.old_pid and .process.automatic_starts == 1
        and (.machine_before | type == "string" and length > 0)
        and .online_machine_after == .machine_before
        and (.enrollment.before | type == "string" and length > 0)
        and .enrollment.after == .enrollment.before'
}

native_terminal_stop() {
    native_check 'The damaged run did not exit once with status 0.' '
        .process.exit_status == 0 and (.process.fault_pid | type == "number" and . > 0)
        and .process.exit_pid == .process.fault_pid' &&
    native_check 'The manager did not stay stopped for 12 seconds before manual recovery.' '
        (.process.stopped_seconds | type == "number" and . >= 12)
        and .process.automatic_starts == 0 and .process.pid_before_recovery == 0' &&
    native_check 'The stopped process did not log exactly one history remedy.' '
        .remedies.history == 1 and .remedies.incompatible == 0' &&
    native_check 'The stopped process changed the disputed bytes or the enrollment.' '
        (.damage.before | type == "string" and length > 0) and .damage.after == .damage.before
        and (.enrollment.before | type == "string" and length > 0)
        and .enrollment.after == .enrollment.before' &&
    native_check 'Restoring the matching history did not bring the same machine back online.' '
        (.recovery.saved_history | type == "string" and length > 0)
        and .recovery.restored_history == .recovery.saved_history
        and (.recovery.pid | type == "number" and . > 0) and .recovery.pid != .process.fault_pid
        and (.machine_before | type == "string" and length > 0)
        and .recovery.online_machine == .machine_before'
}

invalid_history_counter_stops_before_dial() {
    native_check 'The damaged counter was not 2^63 + 1.' '.counter.next == "9223372036854775809"' &&
    native_check 'The invalid counter allowed a dial or an admission.' '
        .link.dials == 0 and .link.admissions == 0' &&
    native_terminal_stop
}

invalid_history_payload_stops_after_welcome() {
    native_check 'The fixture did not keep a required report with valid counters and discard the old relay copy.' '
        def decimal: type == "string" and test("^(0|[1-9][0-9]*)$");
        def below($a; $b): [($a | length), $a] < [($b | length), $b];
        (.mark.before | decimal) and (.payload.number | decimal) and (.counter.next | decimal)
        and below(.mark.before; .payload.number) and below(.payload.number; .counter.next)
        and (below(.counter.next; "9223372036854775808") or .counter.next == "9223372036854775808")
        and (.payload.held_stream | type == "string" and length > 0)
        and .payload.closed_stream == .payload.held_stream and .payload.discarded_stream == .payload.held_stream
        and .payload.forwards_before_fault == 0' &&
    native_check 'The corrupt payload did not reach exactly one verified Welcome on a fresh stream.' '
        .link.dials == 1 and .link.admissions == 1 and .link.verified_welcomes == 1
        and .mark.welcome == .mark.before and (.link.stream | type == "string" and length > 0)
        and .link.stream != .payload.held_stream' &&
    native_check 'The corrupt payload advanced the server mark or reached the server as an event.' '
        .mark.after == .mark.before and .payload.event_handoffs == 0' &&
    native_check 'The corrupt payload changed the Projects records.' '
        (.projects.before | type == "string" and length > 0) and .projects.after == .projects.before' &&
    native_terminal_stop
}

trusted_git_inspects_as_service_user() {
    native_check 'The installed service user did not record a trusted Git path and version.' '
        .checkout.path as $checkout
        | (.service_uid | type == "number" and . > 0) and .git.uid == .service_uid
        and (.git.service_path | type == "string" and length > 0) and .git.probe_path == .git.service_path
        and ($checkout | startswith("/"))
        and (.git.path | startswith("/")) and .git.path == .git.canonical_path
        and .git.path != $checkout and (.git.path | startswith($checkout + "/")) == false
        and (.git.version | test("^git version [0-9]"))' &&
    native_check 'The checkout-local Git was not exercised as a decoy or it ran.' '
        .checkout.path as $checkout
        | (.git.planted_path | startswith($checkout + "/"))
        and (.git.service_path | split(":") | .[0]) == (.git.planted_path | sub("/git$"; ""))
        and (.git.planted_path | endswith("/git")) and .git.planted_invocations == 0' &&
    native_check 'InspectFolder did not return the disposable checkout through the installed machine.' '
        .inspection.http_status == 200 and (.machine_before | type == "string" and length > 0)
        and .inspection.client_id == .machine_before and .inspection.facts.resolvedPath == .checkout.path
        and .inspection.facts.checkedOutBranch == "topic" and .inspection.facts.clean == false'
}

native_fixture_refuses_prepared_stack() {
    local paths=$1 selector
    local -a destinations=("$paths/bin" "$paths/unit" "$paths/log" "$paths/state")
    for selector in COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PROJECT_NAME LGTM_STACK_STATE_DIR LGTM_IMAGE_TAG LGTM_WEB_PORT; do
        if (export "$selector=prepared"; native_fixture_preflight absent "${destinations[@]}") >/dev/null 2>&1; then
            native_reject 'A prepared stack selector passed the native preflight.'
            return 1
        fi
    done
    if (export LGTM_WEB_PORT=8787; native_fixture_preflight absent "${destinations[@]}") >/dev/null 2>&1; then
        native_reject 'The normal installation port passed the native preflight.'
        return 1
    fi
    native_fixture_preflight absent "${destinations[@]}" >/dev/null 2>&1 ||
        native_reject 'A clean fixture with no prepared stack selector was refused.'
}

native_fixture_refuses_existing_installation() {
    local paths=$1 path
    local -a destinations=("$paths/bin" "$paths/unit" "$paths/log" "$paths/state")
    # Use a hard link and a byte comparison to detect a replacement or a write.
    printf '%s\n' 'An existing installation owns this file.' >"$paths/original" &&
        cp -- "$paths/original" "$paths/expected" || return 1
    for path in "${destinations[@]}"; do
        ln -- "$paths/original" "$path" || return 1
        if native_fixture_preflight absent "${destinations[@]}" >/dev/null 2>&1; then
            native_reject 'An occupied installer destination passed the native preflight.'
            return 1
        fi
        [[ $path -ef $paths/original ]] && cmp -s -- "$path" "$paths/expected" ||
            { native_reject 'The preflight changed a foreign installer destination.'; return 1; }
        rm -- "$path" || return 1
    done
    mkdir -- "$paths/state" || return 1
    if native_fixture_preflight absent "${destinations[@]}" >/dev/null 2>&1; then
        native_reject 'An existing state directory passed the native preflight.'
        return 1
    fi
    [[ -d $paths/state ]] && rmdir -- "$paths/state" || return 1
    ln -s -- "$paths/absent" "$paths/bin" || return 1
    if native_fixture_preflight absent "${destinations[@]}" >/dev/null 2>&1; then
        native_reject 'A dangling binary destination passed the native preflight.'
        return 1
    fi
    [[ -L $paths/bin && $(readlink -- "$paths/bin") == "$paths/absent" ]] ||
        { native_reject 'The preflight changed the foreign link.'; return 1; }
    rm -- "$paths/bin" || return 1
    if native_fixture_preflight present "${destinations[@]}" >/dev/null 2>&1 ||
        native_fixture_preflight unknown "${destinations[@]}" >/dev/null 2>&1; then
        native_reject 'An occupied or unreadable manager entry passed the native preflight.'
        return 1
    fi
    native_fixture_preflight absent "${destinations[@]}" >/dev/null 2>&1 ||
        native_reject 'The installer destinations were refused after the controls were removed.'
}

native_report_case() {
    local number=$1 name=$2 manager=$3 reason='' native_observation=''
    shift 3
    if "$@"; then
        printf 'ok %s - %s\n' "$number" "$name"
        return 0
    fi
    printf 'not ok %s - %s # %s: %s\n' "$number" "$name" "$manager" \
        "${reason:-The fixture could not supply the evidence.}"
    return 1
}

native_run_case() {
    local manager=$1 name=$2 candidate=$3
    native_supervisor_observe "$manager" "$name" "$candidate" >/dev/null 2>&1 ||
        { native_reject 'The fixture did not collect the required observation.'; return 1; }
    "$name"
}

native_run_cases() {
    local manager=$1 candidate=$2 number=$3 name status=0
    for name in requested_restart_restarts_service invalid_history_counter_stops_before_dial \
        invalid_history_payload_stops_after_welcome trusted_git_inspects_as_service_user; do
        native_report_case "$number" "$name" "$manager" native_run_case "$manager" "$name" "$candidate" || status=1
        number=$((number + 1))
    done
    return "$status"
}

native_assertions() (
    local scratch manager number=1 status=0
    unset COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PROJECT_NAME LGTM_STACK_STATE_DIR LGTM_IMAGE_TAG LGTM_WEB_PORT
    scratch=$(mktemp -d) || return 1
    trap 'rm -rf -- "$scratch"' EXIT
    printf '1..14\n'
    for manager in systemd launchd; do
        mkdir -- "$scratch/$manager" || return 1
        printf '# %s\n' "$manager"
        native_report_case "$number" native_fixture_refuses_prepared_stack "$manager" \
            native_fixture_refuses_prepared_stack "$scratch/$manager" || status=1
        native_report_case "$((number + 1))" native_fixture_refuses_existing_installation "$manager" \
            native_fixture_refuses_existing_installation "$scratch/$manager" || status=1
        native_report_case "$((number + 2))" native_fixture_owns_stack "$manager" \
            native_run_case "$manager" native_fixture_owns_stack '' || status=1
        native_run_cases "$manager" '' "$((number + 3))" || status=1
        number=$((number + 7))
    done
    return "$status"
)

native_skip() {
    printf '1..0 # SKIP %s\n' "$1"
}

native_manager_entry() {
    local manager=$1 result
    if [[ $manager == systemd ]]; then
        result=$(systemctl --user show --property=LoadState --value lgtm-client.service 2>/dev/null) || :
        case $result in
            not-found) printf 'absent\n' ;;
            loaded|masked|error|bad-setting|stub|merged) printf 'present\n' ;;
            *) printf 'unknown\n' ;;
        esac
    elif result=$(launchctl print "gui/$EUID/dev.lgtmstudio.client" 2>&1); then
        printf 'present\n'
    elif [[ $result == *'Could not find service "dev.lgtmstudio.client" in domain'* ]] &&
        launchctl print "gui/$EUID" >/dev/null 2>&1; then
        printf 'absent\n'
    else
        printf 'unknown\n'
    fi
}

native_live() {
    local candidate=$1 manager tool unit log sessions
    local bin state reason='' native_observation=''
    [[ $EUID != 0 && ${HOME:-} =~ ^/[a-zA-Z0-9._/+@-]+$ && -d $HOME && -O $HOME ]] ||
        { native_skip 'The native cases need a logged-in disposable user with an owned home folder.'; return 0; }
    bin=$HOME/.local/bin/lgtm-client
    state=$HOME/.local/state/lgtm-client
    case $(uname -s) in
        Linux)
            manager=systemd
            unit=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/lgtm-client.service
            log='' # The Linux service writes to the user journal.
            sessions=$(loginctl show-user "$EUID" --property=Sessions --value 2>/dev/null) || sessions=''
            [[ -n $sessions ]] && systemctl --user show-environment >/dev/null 2>&1 &&
                command -v journalctl >/dev/null 2>&1 ||
                { native_skip 'A logged-in systemd user manager and user journal are required.'; return 0; }
            ;;
        Darwin)
            manager=launchd
            unit=$HOME/Library/LaunchAgents/dev.lgtmstudio.client.plist
            log=$HOME/Library/Logs/lgtm-client.log
            launchctl print "gui/$EUID" >/dev/null 2>&1 ||
                { native_skip 'A logged-in launchd graphical user domain is required.'; return 0; }
            ;;
        *) native_skip 'The native cases require Linux with systemd or macOS with launchd.'; return 0 ;;
    esac
    ((BASH_VERSINFO[0] >= 4)) || { native_skip 'The live fixture needs Bash 4 or newer.'; return 0; }
    for tool in docker flock curl jq go git python3; do
        command -v "$tool" >/dev/null 2>&1 ||
            { native_skip "The live fixture needs $tool."; return 0; }
    done
    if ! native_fixture_preflight "$(native_manager_entry "$manager")" "$bin" "$unit" "$log" "$state" >/dev/null 2>&1; then
        printf '%s\n' 'The native fixture refused a prepared stack or an unsafe installer destination.' >&2
        return 1
    fi
    if ! native_run_case "$manager" native_fixture_owns_stack "$candidate"; then
        printf '%s\n' "$reason" >&2
        return 1
    fi
    printf '1..4\n'
    native_run_cases "$manager" "$candidate" 1
}

case "$#:${1:-}" in
    1:--assertions)
        command -v jq >/dev/null 2>&1 || { printf 'The offline assertions need jq.\n' >&2; exit 2; }
        native_assertions
        ;;
    1:[^-]*)
        if [[ ${LGTM_SPRINT4_LIVE:-} == 1 ]]; then
            native_live "$1"
        else
            native_skip 'Set LGTM_SPRINT4_LIVE=1 on the disposable native host to run the installed cases.'
        fi
        ;;
    *) printf 'Usage: bash deploy/test/sprint4-supervisor.sh <candidate-commit> or --assertions\n' >&2; exit 2 ;;
esac
