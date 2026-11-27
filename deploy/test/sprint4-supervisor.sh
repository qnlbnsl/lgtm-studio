#!/usr/bin/env bash
set +x
set +a
set -uo pipefail
umask 077
export LC_ALL=C

# The offline assertions do not run the installer, Docker or a service manager.
# The live observer must keep raw logs and credentials in its owned fixture.

native_fixture_preflight() {
    local selector path
    [[ $# == 5 && $1 == absent ]] || return 1
    for selector in COMPOSE_FILE COMPOSE_ENV_FILES COMPOSE_PROJECT_NAME COMPOSE_PROFILES \
        COMPOSE_DISABLE_ENV_FILE COMPOSE_PATH_SEPARATOR COMPOSE_REMOVE_ORPHANS COMPOSE_IGNORE_ORPHANS \
        COMPOSE_COMPATIBILITY COMPOSE_CONVERT_WINDOWS_PATHS LGTM_STACK_STATE_DIR LGTM_STACK_COMPOSE_SNAPSHOT \
        LGTM_IMAGE_TAG LGTM_WEB_PORT LGTM_WEB_PASSWORD LGTM_CLIENT_STATE_DIR; do
        [[ ! ${!selector+x} ]] || return 1
    done
    [[ $2 == /* && $3 == /* && ( -z $4 || $4 == /* ) && $5 == /* ]] || return 1
    shift
    for path in "$@"; do
        [[ ! -e $path && ! -L $path ]] || return 1
    done
}

native_supervisor_observe() {
    local manager=$1 name=$2 candidate=$3
    native_observation=''
    case $manager in systemd|launchd) ;; *) return 1 ;; esac
    if [[ -z $candidate ]]; then
        native_observation=$(native_offline_records "$name" | native_reduce_observation "$name")
    elif [[ $name == native_fixture_owns_stack && ${LGTM_SPRINT4_LIVE:-} == 1 ]]; then
        native_open_stack "$manager" "$candidate" || return 1
        native_observation=$(native_read_stack | native_reduce_observation "$name")
    else
        # The integrated candidate needs native process, transport and store collectors.
        return 1
    fi
}

# Read the same Compose selection the installer uses, after checking the listener.
native_read_stack() {
    local owned_web current_web binding project daemon
    sprint4_fixture_ready || return 1
    owned_web=$(awk -v name="$s4_project-web-1" '$1 == "container" && $2 == name {print $3}' "$s4_dir/objects") &&
        [[ $owned_web =~ ^[0-9a-f]{64}$ ]] &&
        current_web=$("$s4_docker" compose ps --status running --quiet web) &&
        binding=$("$s4_docker" compose port web 8080) &&
        project=$("$s4_docker" compose config --format json | jq -er '.name') &&
        daemon=$(s4_daemon) && sprint4_fixture_ready || return 1
    jq -nc --arg project "$s4_project" --arg image_tag "$s4_tag" --arg state "$s4_state" \
        --arg checkout "$s4_candidate" --arg ownership_record "$s4_dir/objects" \
        --argjson reserved "$native_reserved_inventory" --argjson port "$s4_port" \
        --arg daemon "$s4_daemon_id" --arg container "$owned_web" \
        --arg installer_project "$project" --arg installer_daemon "$daemon" \
        --arg installer_container "$current_web" --arg binding "$binding" '
        {reservation: {$project, $image_tag, $state, $checkout, $ownership_record, $reserved, $port, $daemon},
         owned_web: {$container},
         installer: {project: $installer_project, daemon: $installer_daemon,
                     container: $installer_container, $binding}}'
}

native_open_stack() {
    local manager=$1 candidate=$2 root path parent
    # The installer destinations are resolved by the native entry point.
    [[ $(cd -- "$HOME" && pwd -P) == "$HOME" ]] || return 1
    for path in "$bin" "$unit" "$log" "$state"; do
        [[ -n $path ]] || continue
        [[ $path == "$HOME/"* && $path != */../* && $path != */./* && $path != *//* &&
            $path =~ ^/[a-zA-Z0-9._/+@-]+$ ]] || return 1
        parent=${path%/*}
        while [[ $parent != "$HOME" ]]; do
            [[ $parent == "$HOME/"* && ! -L $parent &&
                ( ! -e $parent || ( -d $parent && -O $parent ) ) ]] || return 1
            parent=${parent%/*}
        done
    done
    native_fixture_preflight "$(native_manager_entry "$manager")" "$bin" "$unit" "$log" "$state" || return 1
    root=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P) || return 1
    source "$root/deploy/test/sprint4-fixture.sh" &&
        sprint4_fixture_open "$root" "$candidate" &&
        native_reserved_inventory=$(awk 'NF {print}' "$s4_dir/objects" | jq -Rsc 'split("\n") | map(select(length > 0))') &&
        sprint4_select 4 &&
        sprint4_fixture_run bash "$S4_CANDIDATE_ROOT/deploy/up.sh"
}

native_reduce_observation() {
    jq -ce --arg name "$1" '
        def count($kind): [.records[] | select(.kind == $kind)] | length;
        def starts_during($from; $through):
            [.processes[] | select(.action == "start" and $from < .at and .at <= $through)] | length;
        def process_at($action; $at):
            [.processes[] | select(.action == $action and .at == $at)] | if length == 1 then .[0] else null end;
        def terminal($run):
            . as $input
            | process_at("start"; $run.started) as $fault
            | process_at("exit"; $run.exited) as $exit
            | (process_at("start"; $run.recovered) | select(.at > $run.checked)) as $recovery
            | {machine_before: .machine_before, enrollment: .enrollment, damage: .damage,
               process: {fault_pid: $fault.pid, exit_pid: $exit.pid, exit_status: $exit.status,
                         stopped_seconds: ($run.checked - $exit.at), pid_before_recovery: $run.checked_pid,
                         automatic_starts: starts_during($fault.at; $run.checked)},
               remedies: (.logs | map(select(.pid == $fault.pid and $fault.at <= .at and .at <= $exit.at))
                          | {history: map(select(.diagnostic == "history")) | length,
                             incompatible: map(select(.diagnostic == "incompatible")) | length}),
               recovery: ($input.recovery + {pid: $recovery.pid})};
        def during_fault:
            .fault as $fault | .records |= map(select($fault.started <= .at and .at <= $fault.checked));
        if $name == "native_fixture_owns_stack" then
            {fixture: ((.reservation | del(.reserved)) + {
                preexisting_resources: (.reservation.reserved | length),
                web_container: .owned_web.container, binding: ("127.0.0.1:" + (.reservation.port | tostring)),
                installer_daemon: .installer.daemon, installer_project: .installer.project,
                installer_binding: .installer.binding, bound_container: .installer.container})}
        elif $name == "requested_restart_restarts_service" then
            process_at("start"; .request.prior_start) as $old
            | process_at("exit"; .request.exit) as $exit
            | .online.checked as $checked
            | (process_at("start"; .online.start) | select($exit.at < .at and .at <= $checked)) as $replacement
            | {reconnect_status: .request.status, enrollment: .enrollment,
               machine_before: .request.machine, online_machine_after: .online.machine,
               process: {old_pid: $old.pid, exit_pid: $exit.pid, exit_status: $exit.status,
                         replacement_pid: $replacement.pid,
                         automatic_starts: starts_during($old.at; .online.checked)}}
        elif $name == "invalid_history_counter_stops_before_dial" then
            during_fault | terminal(.fault) + {counter: .store.meta,
                link: {dials: count("dial"), admissions: count("admission")}}
        elif $name == "invalid_history_payload_stops_after_welcome" then
            .fault.started as $fault_start
            | def one_stream($action; match_record):
                [.relay[] | select(.at < $fault_start and .action == $action and match_record) | .stream_id]
                | if length == 1 then .[0] else null end;
            .store.event_number as $number
            | one_stream("held"; .frame == "event" and .event_number == $number) as $held
            | .mark as $mark
            | during_fault
            | [.records[] | select(.kind == "verified_welcome")] as $welcomes
            | terminal(.fault) + {counter: .store.meta, projects: .projects,
                mark: ($mark + {welcome: $welcomes[0].last_event_number}),
                link: {dials: count("dial"), admissions: count("admission"),
                       verified_welcomes: ($welcomes | length), stream: $welcomes[0].stream_id},
                payload: {number: $number, held_stream: $held,
                          closed_stream: one_stream("closed"; .stream_id == $held),
                          discarded_stream: one_stream("discarded"; .stream_id == $held and
                              .frame == "event" and .event_number == $number),
                          forwards_before_fault: ([.relay[] | select(.action == "forwarded" and
                              .frame == "event" and .event_number == $number and .at < $fault_start)] | length),
                          event_handoffs: count("event_handoff")}}
        elif $name == "trusted_git_inspects_as_service_user" then
            .probe.planted_path as $decoy
            | (.response.facts | if type == "object" and (has("clean") | not) then .clean = false else . end) as $facts
            | {service_uid: .service.uid, checkout: {path: .checkout}, machine_before: .service.machine,
             git: ((.probe | del(.invocations)) + {service_path: .service.path,
                   planted_invocations: ([.probe.invocations[] | select(. == $decoy)] | length)}),
             inspection: {http_status: .request.status, client_id: .request.client_id, facts: $facts}}
        else error("Unknown native observation.")
        end'
}

# Replay synthetic source records on both managers in the offline lane.
# The records contain no enrollment data, credentials or native host output.
native_offline_records() {
    local name=$1
    case $name in
        native_fixture_owns_stack)
            cat <<'JSON'
{
  "reservation": {"project":"lgtm-s4-offline","image_tag":"lgtm-s4-offline-4","state":"/fixture/state",
                  "checkout":"/fixture/candidate","ownership_record":"/fixture/objects",
                  "reserved":[],"port":32769,"daemon":"fixture-daemon"},
  "owned_web": {"container":"fixture-web"},
  "installer": {"project":"lgtm-s4-offline","daemon":"fixture-daemon",
                "container":"fixture-web","binding":"127.0.0.1:32769"}
}
JSON
            ;;
        requested_restart_restarts_service)
            cat <<'JSON'
{
  "request":{"status":200,"prior_start":1,"exit":3,"machine":"fixture-machine"},
  "online":{"start":8,"checked":10,"machine":"fixture-machine"},
  "enrollment":{"before":"fixture-enrollment","after":"fixture-enrollment"},
  "processes":[{"action":"start","at":1,"pid":20},{"action":"exit","at":3,"pid":20,"status":75},
               {"action":"start","at":8,"pid":31}]
}
JSON
            ;;
        invalid_history_counter_stops_before_dial|invalid_history_payload_stops_after_welcome)
            native_offline_history "$name"
            ;;
        trusted_git_inspects_as_service_user)
            cat <<'JSON'
{
  "service":{"uid":501,"path":"/fixture/repo-b/decoy:/usr/bin","machine":"fixture-machine"},
  "checkout":"/fixture/repo-b",
  "probe":{"uid":501,"probe_path":"/fixture/repo-b/decoy:/usr/bin","path":"/usr/bin/git",
           "canonical_path":"/usr/bin/git","version":"git version 2.0","planted_path":"/fixture/repo-b/decoy/git",
           "invocations":["/usr/bin/git"]},
  "request":{"status":200,"client_id":"fixture-machine"},
  "response":{"facts":{"resolvedPath":"/fixture/repo-b","checkedOutBranch":"topic"}}
}
JSON
            ;;
        *) return 1 ;;
    esac
}

native_offline_history() {
    local name=$1
    jq -c --arg name "$name" '
        if $name == "invalid_history_counter_stops_before_dial" then
            .store.meta.next = "9223372036854775809"
        else
            .store = {meta: {next: "43"}, event_number: "42"}
            | .mark = {before: "41", after: "41"}
            | .projects = {before: "fixture-projects", after: "fixture-projects"}
            | .records += [{kind: "dial", at: 3}, {kind: "admission", at: 4},
                {kind: "verified_welcome", at: 5, stream_id: "new-stream", last_event_number: "41"}]
            | .relay = [{action: "closed", stream_id: "unrelated-stream", at: 0},
                {action: "held", frame: "event", event_number: "42", stream_id: "old-stream", at: 0},
                {action: "discarded", frame: "event", event_number: "42", stream_id: "old-stream", at: 1},
                {action: "closed", stream_id: "old-stream", at: 1}]
        end' <<'JSON'
{
  "fault":{"started":2,"exited":6,"checked":18,"checked_pid":0,"recovered":19},
  "machine_before":"fixture-machine",
  "enrollment":{"before":"fixture-enrollment","after":"fixture-enrollment"},
  "damage":{"before":"fixture-damage","after":"fixture-damage"},
  "recovery":{"saved_history":"fixture-history","restored_history":"fixture-history","online_machine":"fixture-machine"},
  "processes":[{"action":"start","at":2,"pid":20},{"action":"exit","at":6,"pid":20,"status":0},
               {"action":"start","at":19,"pid":31}],
  "logs":[{"pid":20,"at":5,"diagnostic":"history"}],
  "records":[{"kind":"dial","at":19},{"kind":"admission","at":20}]
}
JSON
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
