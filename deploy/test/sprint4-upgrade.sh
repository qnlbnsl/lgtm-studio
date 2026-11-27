#!/usr/bin/env bash
set +x
set +a
set -uo pipefail
umask 077
export LC_ALL=C

upgrade_observe() {
    # The installed collector needs the integrated candidate.
    [[ -z $2 ]] || return 1
    upgrade_observation=$(upgrade_offline_observation)
}

upgrade_filters='
    def text: type == "string" and length > 0;
    def digest: type == "string" and test("^[a-f0-9]{64}$");
    def uint: type == "number" and . >= 0 and . <= 9007199254740991 and floor == .;
    def positive: uint and . > 0;
    def identity:
        . as $id | ($id.project | type == "string" and test("^lgtm-s4-[a-z0-9_-]+$"))
        and ($id.port | positive and . <= 65535 and . != 8787)
        and $id.binding == ("127.0.0.1:" + ($id.port | tostring))
        and all([$id.daemon, $id.state, $id.volume, $id.bao_store, $id.password_source,
                 $id.machine][]; text)
        and all([$id.password_fingerprint, $id.session, $id.identity, $id.key, $id.vault_key][]; digest)
        and $id.approved == true;
    def same_installation($before; $after):
        ($before | identity)
        and ([$before, $after] | map({project, port, binding, daemon, state, volume, bao_store,
            password_source, password_fingerprint, session, identity, key, vault_key,
            machine, approved}) | .[0] == .[1]);
    def website_ready:
        .healthy == true and .get_server.signedIn == true
        and .get_server.product == "lgtm-studio" and .get_server.sample != true
        and .get_server.contract == 1
        and (.get_server.services | type == "array" and index("lgtm.v1.ProjectService") != null);
    def disk_record:
        (.event_number | positive) and (.digest | digest) and (.command_id | text)
        and (.outcome | positive and . <= 5);
    def frames($machine; select_frame):
        [.relay | to_entries[] | select(.value.machine_id == $machine and (.value | select_frame))];
    def one: if length == 1 then .[0] else null end;
    def ledger:
        type == "object" and length > 0 and all(.[]; positive);
    def reviewed_migrations($old; $candidate):
        ($old | ledger) and ($candidate | ledger) and $old != $candidate
        and $candidate.link >= 3 and $candidate.store >= 2 and $candidate.projects >= 1
        and all($old | to_entries[]; .key as $part | $candidate[$part] >= .value);
    def pair: (.server | digest) and (.web | digest);
'

upgrade_check() {
    local message=$1 filter=$2
    if ! jq -es "$upgrade_filters length == 1 and (.[0] | $filter)" <<<"$upgrade_observation" >/dev/null 2>&1; then
        upgrade_reason=$message
        return 1
    fi
}

verified_revision_one_refusal() {
    upgrade_check 'Cutover did not verify the owned selection, original password source, private backups and prebuilt pair before touching the installation.' '
        (.original | identity)
        and (.preflight.ownership_record | text) and .preflight.actions_before_ownership == 0
        and .preflight.password_fingerprint == .original.password_fingerprint
        and all([.preflight.backups.postgres, .preflight.backups.stack, .preflight.backups.client][]; digest)
        and (.preflight.candidate_images | pair)
        and (.original.binary | digest)
        and .preflight.old_website_serving == true
        and all([.preflight.started, .preflight.password_verified, .preflight.backups_verified,
                 .preflight.client_quiesced, .preflight.images_ready, .preflight.cutover_started][]; positive)
        and .preflight.started <= .preflight.password_verified
        and .preflight.started <= .preflight.backups_verified
        and .preflight.started <= .preflight.images_ready
        and .preflight.started <= .preflight.client_quiesced
        and .preflight.client_quiesced <= .preflight.backups_verified
        and .preflight.password_verified < .preflight.cutover_started
        and .preflight.backups_verified < .preflight.cutover_started
        and .preflight.images_ready < .preflight.cutover_started' &&
    upgrade_check 'The candidate did not migrate the original installation and return Projects with the original session while the client was offline.' '
        same_installation(.original; .cutover)
        and reviewed_migrations(.original.ledger; .candidate_manifest)
        and .cutover.ledger == .candidate_manifest
        and .cutover.images == .preflight.candidate_images
        and (.cutover | website_ready)
        and .cutover.online == false' &&
    upgrade_check 'The old installed binary did not verify a signed Challenge and receive the revision 1 refusal before Welcome or admission.' '
        .original.machine as $machine | .refusal as $r
        | frames($machine; .stream_id == $r.stream_id and .connection_id == $r.connection_id) as $leg
        | ($leg | map(select(.value.action == "forwarded" and .value.direction == "up"
            and .value.frame == "open" and .value.link_revision == 1)) | one) as $open
        | ($leg | map(select(.value.action == "forwarded" and .value.direction == "down"
            and .value.frame == "challenge")) | one) as $challenge
        | ($leg | map(select(.value.action == "forwarded" and .value.direction == "up"
            and .value.frame == "proof")) | one) as $proof
        | ($leg | map(select(.value.action == "closed")) | one) as $closed
        | ($r.stream_id | text) and ($r.connection_id | positive)
        and $r.binary == .original.binary and $r.stream_id != .old.stream_id
        and $open != null and $challenge != null and $proof != null and $closed != null
        and $open.key < $proof.key and $proof.key < $closed.key and $challenge.key < $closed.key
        and $r.verified_challenge == true
        and $r.code == "failed_precondition" and $r.trailer == ["link_revision_mismatch"]
        and ([$leg[] | select(.value.frame == "welcome")] | length == 0)
        and $r.admissions == 0 and $r.quiesced == true and $r.candidate_started == false
        and (.disk | disk_record) and $r.disk == .disk'
}

installed_state_retained() {
    upgrade_check 'The installer rerun did not keep the original stores, password source, authenticated session, approved identity and keys.' '
        same_installation(.original; .installed)
        and (.installed | website_ready)
        and .installed.online == true
        and (.installed.running_binary | digest) and (.original.binary | digest)
        and .installed.binary == .installed.running_binary and .installed.binary != .original.binary
        and (.installed.mode.directory | type == "number" and . == 448)
        and all([.installed.mode.db, .installed.mode.identity, .installed.mode.key][]; . == 384)' &&
    upgrade_check 'The installer did not reuse enrollment without issuing a token or making a Join request.' '
        .installer.status == 0 and (.installer.messages | type == "array" and
            index("lgtm-client is already joined.") != null)
        and .installer.tokens_issued == 0 and .installer.joins == 0' &&
    upgrade_check 'The settled sentinel command did not survive the upgrade.' '
        .original.sentinel as $old
        | ($old.command_id | text) and $old.machine_id == .original.machine
        and ($old.attempt | positive) and $old.outcome == "done" and ($old.finished_at | text)
        and .installed.sentinel == $old'
}

retained_event_and_receipt_replayed() {
    upgrade_check 'The old scan did not leave its exact buffered report and finished receipt in the closed store while both server records remained unsettled.' '
        .disk as $disk | .old as $old
        | ($disk | disk_record) and $disk.outcome == 1 and ($old.stream_id | text)
        and ($old.connection_id | positive) and ($old.generation | positive)
        and ($old.prefix_mark | positive) and $disk.event_number == $old.prefix_mark + 1
        and $old.mark_before_cutover == $old.prefix_mark
        and $old.scan_rpc == "ClientService.ScanClientHarnesses"
        and ($old.attempt | positive) and $old.unsettled.id == $disk.command_id
        and $old.unsettled.machine_id == .original.machine and $old.unsettled.attempt == $old.attempt
        and ($old.unsettled | has("finished_at") and .finished_at == null and has("outcome") and .outcome == null)
        and $old.service_stopped == true and $old.lock_released == true
        and ($old.scan_invocations | positive)' &&
    upgrade_check 'The relay did not retire the exact old event and result holds without forwarding either copy.' '
        .disk as $disk | .old as $old | .original.machine as $machine
        | frames($machine; .stream_id == $old.stream_id and .connection_id == $old.connection_id) as $leg
        | def placed($action; match):
            $leg | map(select(.value.action == $action and .value.generation == $old.generation
                and (.value | match))) | one;
        def event_match: .direction == "up" and .frame == "event" and .event_number == $disk.event_number;
        def result_match: .direction == "up" and .frame == "command_result"
                and .command_id == $disk.command_id and .attempt == $old.attempt;
        ($leg | map(select(.value.action == "forwarded" and .value.frame == "event_ack" and
            .value.direction == "down" and .value.event_number == $old.prefix_mark)) | one) as $prefix_ack
        | ($leg | map(select(.value.action == "forwarded" and .value.frame == "command" and
            .value.direction == "down" and .value.command_id == $disk.command_id and
            .value.attempt == $old.attempt)) | one) as $command
        | (placed("armed"; event_match)) as $event_rule
        | (placed("armed"; result_match)) as $result_rule
        | (placed("held"; event_match)) as $event
        | (placed("held"; result_match)) as $result
        | (placed("discarded"; event_match)) as $discard_event
        | (placed("discarded"; result_match)) as $discard_result
        | (placed("closed"; true)) as $closed
        | all([$prefix_ack, $command, $event_rule, $result_rule, $event, $result,
            $discard_event, $discard_result, $closed][]; . != null)
        and $prefix_ack.key < $command.key and $command.key < $event_rule.key and $command.key < $result_rule.key
        and $event_rule.key < $event.key and $result_rule.key < $result.key
        and $event.key < $discard_event.key and $result.key < $discard_result.key
        and $discard_event.key < $closed.key and $discard_result.key < $closed.key
        and $event.value.digest == $disk.digest and $discard_event.value.digest == $disk.digest
        and $result.value.outcome == $disk.outcome and $discard_result.value.outcome == $disk.outcome
        and ([$leg[] | .value | select(.action == "forwarded" and (event_match or result_match))]
            | length == 0)' &&
    upgrade_check 'A fresh revision 2 stream did not replay the disk digest, receive its cumulative acknowledgement and advance the durable mark once.' '
        .disk as $disk | .old as $old | .replay as $new | .original.machine as $machine
        | frames($machine; .stream_id == $new.stream_id and .connection_id == $new.connection_id
            and .action == "forwarded") as $leg
        | (frames($machine; .action == "closed" and .stream_id == $old.stream_id) | one) as $closed
        | ($leg | map(select(.value.frame == "open" and .value.direction == "up" and
            .value.link_revision == 2)) | one) as $open
        | ($leg | map(select(.value.frame == "welcome" and .value.direction == "down" and
            .value.link_revision == 2 and .value.last_event_number == $old.mark_before_cutover)) | one) as $welcome
        | (frames($machine; .action == "forwarded" and .frame == "event" and .direction == "up"
            and .event_number == $disk.event_number) | one) as $event
        | ($leg | map(select(.value.frame == "event_ack" and .value.direction == "down" and
            .value.event_number == $disk.event_number)) | one) as $ack
        | ($new.stream_id | text) and ($new.connection_id | positive)
        and $new.stream_id != $old.stream_id and $new.stream_id != .refusal.stream_id
        and all([$closed, $open, $welcome, $event, $ack][]; . != null)
        and $closed.key < $open.key and $closed.key < $welcome.key
        and $open.key < $event.key and $welcome.key < $ack.key
        and $event.value.stream_id == $new.stream_id
        and $event.value.connection_id == $new.connection_id and $event.value.digest == $disk.digest
        and ($new.mark_after_ack | uint) and $new.mark_after_ack >= $disk.event_number
        and ([.applications[] | select(.message == "link_event_applied" and
            .machine_id == $machine and .sequence == $disk.event_number)] | length == 1)' &&
    upgrade_check 'The old receipt did not settle once with its saved outcome and zero watermark on the reoffered attempt.' '
        .disk as $disk | .old as $old | .replay as $new | .original.machine as $machine
        | frames($machine; .action == "forwarded" and .command_id == $disk.command_id) as $command_frames
        | ($command_frames | map(select(.value.frame == "command" and .value.direction == "down"
            and .value.stream_id == $new.stream_id and .value.connection_id == $new.connection_id)) | one) as $offer
        | ($command_frames | map(select(.value.frame == "command_result" and .value.direction == "up")) | one) as $result
        | ([.settlements[] | select(.id == $disk.command_id and .machine_id == $machine)] | one) as $settled
        | $offer != null and $result != null
        and ($offer.value.attempt | positive) and $offer.value.attempt > $old.attempt
        and $result.value.stream_id == $new.stream_id and $result.value.connection_id == $new.connection_id
        and $result.value.attempt == $offer.value.attempt
        and $result.value.outcome == $disk.outcome and $result.value.last_event_number == 0
        and all([$old.expires_at, $new.reoffered_at, $new.settled_at][]; positive)
        and $new.reoffered_at <= $new.settled_at and $new.settled_at < $old.expires_at + 82800
        and $settled != null and $settled.attempt == $offer.value.attempt
        and $settled.outcome == "done" and ($settled.finished_at | text)
        and .settled_after_control == $settled' &&
    upgrade_check 'The scan probe did not separate startup inventory, zero replay executions and a fresh positive-control scan.' '
        (.probe.startup_invocations | uint)
        and .probe.replay_invocations == 0 and (.probe.control_invocations | positive)
        and (.probe.control_command_id | text) and .probe.control_command_id != .disk.command_id
        and .probe.control_machine_id == .original.machine and .probe.control_outcome == "done"'
}

upgrade_recovers_forward() {
    upgrade_check 'Recovery did not interrupt and observe each of the three planned boundaries.' '
        (.recoveries | type == "array" and length == 3)
        and ([.recoveries[].boundary] | sort) ==
            ["after_binary_before_restart", "after_migration_before_web", "before_migration_commit"]
        and all(.recoveries[]; .fault_observed == true and .client_quiesced == true
            and .client_starts_before_ledger_verified == 0
            and (.boundary == "after_binary_before_restart" or .candidate_stopped_before_ledger == true))' &&
    upgrade_check 'Recovery did not keep the original volume, session, approved machine and keys on the healthy candidate.' '
        all(.recoveries[];
            same_installation(.original; .during)
            and same_installation(.original; .recovered)
            and (.recovered | website_ready) and .recovered.online == true
            and (.legacy_images | pair) and (.candidate_images | pair) and .legacy_images != .candidate_images
            and reviewed_migrations(.original.ledger; .candidate_manifest)
            and .recovered.ledger == .candidate_manifest and .recovered.images == .candidate_images
            and (.restart_images | type == "array" and length > 0 and all(.[]; pair)))' &&
    upgrade_check 'Recovery used the wrong launcher for the verified migration ledger or did not rerun the installer after replacement.' '
        all(.recoveries[];
            . as $case
            | if .boundary == "before_migration_commit" then
                .during.ledger == .original.ledger and .during.web_ready == false
                and .restart_images[0] == .legacy_images and .restart_images[-1] == .candidate_images
                and .old_session_recovered == true
            elif .boundary == "after_migration_before_web" then
                .during.ledger == .candidate_manifest and .during.web_ready == false
                and all(.restart_images[]; . == $case.candidate_images)
            else
                .during.ledger == .candidate_manifest and .during.web_ready == true
                and all(.restart_images[]; . == $case.candidate_images)
                and (.original.binary | digest) and (.during.binary | digest)
                and .during.binary != .original.binary and .during.replacement_started == false
                and (.installer.interrupted | digest) and .installer.rerun == .installer.interrupted
                and .installer.rerun_status == 0 and .recovered.binary == .during.binary
                and .recovered.running_binary == .recovered.binary
            end)'
}

upgrade_run_cases() {
    local candidate=$1 name number=0 status=0 upgrade_reason upgrade_observation
    printf '1..4\n'
    for name in verified_revision_one_refusal installed_state_retained \
        retained_event_and_receipt_replayed upgrade_recovers_forward; do
        number=$((number + 1))
        upgrade_reason=''
        upgrade_observation=''
        if upgrade_observe "$name" "$candidate" >/dev/null 2>&1 && "$name"; then
            printf 'ok %s - %s\n' "$number" "$name"
        else
            printf 'not ok %s - %s # %s\n' "$number" "$name" \
                "${upgrade_reason:-The fixture did not collect the required observation.}"
            status=1
        fi
    done
    return "$status"
}

upgrade_skip() {
    printf '1..0 # SKIP %s\n' "$1"
}

upgrade_main() {
    if [[ $# != 1 || -z $1 || ( $1 == -* && $1 != --assertions ) ]]; then
        printf 'Usage: bash deploy/test/sprint4-upgrade.sh <candidate-commit> or --assertions\n' >&2
        return 2
    fi
    if [[ $1 == --assertions ]]; then
        command -v jq >/dev/null 2>&1 || { printf 'The offline assertions need jq.\n' >&2; return 2; }
        upgrade_run_cases ''
        return
    fi
    if [[ ${LGTM_SPRINT4_LIVE:-} != 1 ]]; then
        upgrade_skip 'Set LGTM_SPRINT4_LIVE=1 on the disposable Linux service host to run the installed upgrade.'
        return
    fi
    if [[ $(uname -s) != Linux || $EUID == 0 ]] ||
        ! command -v journalctl >/dev/null 2>&1 ||
        ! systemctl --user show-environment >/dev/null 2>&1; then
        upgrade_skip 'The installed upgrade needs a disposable Linux user, a running systemd user manager and the journal.'
        return
    fi
    command -v jq >/dev/null 2>&1 || { upgrade_skip 'The installed upgrade needs jq.'; return; }
    upgrade_run_cases "$1"
}

# These synthetic records exercise the assertions without an installed service.
upgrade_offline_observation() {
    jq -nc '
        def digest($digit): $digit * 64;
        def pair($server; $web): {server: digest($server), web: digest($web)};
        def frame($stream; $connection; $action; $direction; $frame):
            {stream_id: $stream, connection_id: $connection, $action, $direction, $frame,
             machine_id: "offline-machine"};
        def old_frame($action; $direction; $frame):
            frame("old"; 1; $action; $direction; $frame) + {generation: 1};
        def refusal_frame($action; $direction; $frame):
            frame("refusal"; 2; $action; $direction; $frame);
        def new_frame($direction; $frame): frame("new"; 3; "forwarded"; $direction; $frame);
        def discovery:
            {signedIn: true, product: "lgtm-studio", contract: 1,
             services: ["lgtm.v1.ProjectService"]};

        {project: "lgtm-s4-offline", port: 18787, binding: "127.0.0.1:18787",
         daemon: "offline-daemon", state: "offline-state", volume: "offline-volume",
         bao_store: "offline-bao", password_source: "offline-settings",
         password_fingerprint: digest("1"), session: digest("2"),
         identity: digest("3"), key: digest("4"), vault_key: digest("5"),
         machine: "offline-machine", approved: true} as $identity
        | pair("6"; "7") as $legacy_images
        | pair("8"; "9") as $candidate_images
        | {link: 2, store: 1} as $legacy_ledger
        | {link: 3, store: 2, projects: 1} as $candidate_manifest
        | {command_id: "offline-sentinel", machine_id: $identity.machine, attempt: 1,
           outcome: "done", finished_at: "2026-11-27T12:00:00Z"} as $sentinel
        | ($identity + {binary: digest("a"), ledger: $legacy_ledger, sentinel: $sentinel}) as $original
        | ($identity + {healthy: true, get_server: discovery, online: false,
            ledger: $candidate_manifest, images: $candidate_images}) as $cutover
        | ($cutover + {online: true, binary: digest("b"), running_binary: digest("b"),
            mode: {directory: 448, db: 384, identity: 384, key: 384},
            sentinel: $sentinel}) as $installed
        | {event_number: 42, digest: digest("c"), command_id: "offline-scan", outcome: 1} as $disk
        | ($disk | {event_number, digest}) as $event
        | ($disk | {command_id, outcome} + {attempt: 1}) as $result
        | {id: $disk.command_id, machine_id: $identity.machine, attempt: 2,
           outcome: "done", finished_at: "2026-11-27T12:02:00Z"} as $settled
        | {fault_observed: true, client_quiesced: true, client_starts_before_ledger_verified: 0,
           candidate_stopped_before_ledger: true, $original, recovered: $installed,
           $legacy_images, $candidate_images, $candidate_manifest} as $recovery
        | {
            $original, $cutover, $installed, $candidate_manifest, $disk,
            preflight: {
                ownership_record: "offline-ownership", actions_before_ownership: 0,
                password_fingerprint: $identity.password_fingerprint, old_website_serving: true,
                backups: {postgres: digest("d"), stack: digest("e"), client: digest("f")},
                $candidate_images, started: 1, password_verified: 2, client_quiesced: 3,
                backups_verified: 4, images_ready: 5, cutover_started: 6
            },
            refusal: {
                stream_id: "refusal", connection_id: 2, binary: $original.binary,
                verified_challenge: true, code: "failed_precondition", trailer: ["link_revision_mismatch"],
                admissions: 0, quiesced: true, candidate_started: false, $disk
            },
            installer: {status: 0, messages: ["lgtm-client is already joined."], tokens_issued: 0, joins: 0},
            old: {
                stream_id: "old", connection_id: 1, generation: 1,
                prefix_mark: 41, mark_before_cutover: 41, scan_rpc: "ClientService.ScanClientHarnesses",
                attempt: 1, unsettled: ($settled + {attempt: 1, finished_at: null, outcome: null}),
                service_stopped: true, lock_released: true, scan_invocations: 1,
                expires_at: ("2026-11-27T12:03:00Z" | fromdateiso8601)
            },
            replay: {
                stream_id: "new", connection_id: 3, mark_after_ack: $disk.event_number,
                reoffered_at: ("2026-11-27T12:01:59Z" | fromdateiso8601),
                settled_at: ($settled.finished_at | fromdateiso8601)
            },
            relay: [
                (old_frame("forwarded"; "down"; "event_ack") + {event_number: 41}),
                (old_frame("forwarded"; "down"; "command") + ($result | {command_id, attempt})),
                (old_frame("armed"; "up"; "event") + ($event | {event_number})),
                (old_frame("armed"; "up"; "command_result") + ($result | {command_id, attempt})),
                (old_frame("held"; "up"; "event") + $event),
                (old_frame("held"; "up"; "command_result") + $result),
                (old_frame("discarded"; "up"; "event") + $event),
                (old_frame("discarded"; "up"; "command_result") + $result),
                old_frame("closed"; null; null),
                (refusal_frame("forwarded"; "up"; "open") + {link_revision: 1}),
                refusal_frame("forwarded"; "down"; "challenge"),
                refusal_frame("forwarded"; "up"; "proof"),
                refusal_frame("closed"; null; null),
                (new_frame("up"; "open") + {link_revision: 2}),
                (new_frame("down"; "welcome") + {link_revision: 2, last_event_number: 41}),
                (new_frame("up"; "event") + $event),
                (new_frame("down"; "event_ack") + ($event | {event_number})),
                (new_frame("down"; "command") + ($disk | {command_id}) + {attempt: 2}),
                (new_frame("up"; "command_result") + $result + {attempt: 2, last_event_number: 0})
            ],
            applications: [{message: "link_event_applied", machine_id: $identity.machine,
                            sequence: $disk.event_number}],
            settlements: [$settled], settled_after_control: $settled,
            probe: {
                startup_invocations: 1, replay_invocations: 0, control_invocations: 1,
                control_command_id: "offline-control", control_machine_id: $identity.machine,
                control_outcome: "done"
            },
            recoveries: [
                ($recovery + {boundary: "before_migration_commit",
                    during: ($original + {web_ready: false}),
                    restart_images: [$legacy_images, $candidate_images], old_session_recovered: true}),
                ($recovery + {boundary: "after_migration_before_web",
                    during: ($cutover + {web_ready: false}), restart_images: [$candidate_images]}),
                ($recovery + {boundary: "after_binary_before_restart",
                    during: ($installed + {web_ready: true, replacement_started: false}),
                    restart_images: [$candidate_images],
                    installer: {interrupted: digest("0"), rerun: digest("0"), rerun_status: 0}})
            ]
        }'
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
    upgrade_main "$@"
fi
