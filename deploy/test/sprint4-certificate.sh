#!/usr/bin/env bash
set +x
set +a
set -u -o pipefail
umask 077
export LC_ALL=C

# Each phase records an independently observed listener, the leaf seen by a
# fresh handshake, the client result, and that phase's relay observation window.
# Steps are collection order markers. The leaf probe completes before the client
# starts, and a phase ends after its clients stop and its observations are read.
certificate_filters='
    def text: type == "string" and length > 0;
    def digest: type == "string" and test("^[0-9a-f]{64}$");
    def positive: type == "number" and . > 0 and floor == .;
    def origin:
        "https://" + .alias + (if .port == 443 then "" else ":" + (.port | tostring) end);
    def listener($network; $caddy):
        (.id | text) and .network_id == $network and .upstream_id == $caddy
        and (.alias | type == "string" and test("^[a-zA-Z][a-zA-Z0-9.-]*$")
            and (ascii_downcase | rtrimstr(".") | split(".") | last != "localhost"))
        and (.port | positive and . <= 65535) and .added_published_ports == [];
    def leaf:
        .valid_for_alias == true and (.certificate_sha256 | digest) and (.spki_sha256 | digest)
        and (.observed_step | positive);
    def phase:
        (.leaf | leaf)
        and (.join.started_step | positive) and (.completed_step | positive)
        and .leaf.observed_step < .join.started_step and .join.started_step < .completed_step
        and (.relay | type == "array" and all(.[]; (.action | text)));
    def frame($direction; $frame):
        .action == "forwarded" and .direction == $direction and .frame == $frame;
    def connection:
        (.connection_id | positive) and (.stream_id | text);
    def same_open($open):
        .stream_id == $open.stream_id and .connection_id == $open.connection_id
        and .machine_id == $open.machine_id;
    def matching_control:
        . as $phase | .join.machine_id as $machine
        | .join.status == 0 and ($machine | text)
        and ([.relay[] | select(frame("up"; "join"))]
            | length == 1 and (.[0] | connection and .machine_id == $machine))
        and any(.relay[];
            select(frame("up"; "open") and connection and .link_revision == 2
                and .machine_id == $machine) as $open
            | any($phase.relay[]; frame("down"; "welcome") and same_open($open) and .link_revision == 2));
'

certificate_check() {
    local message=$1 filter=$2
    if ! jq -es "$certificate_filters length == 1 and (.[0] | $filter)" \
        <<<"$certificate_observation" >/dev/null 2>&1; then
        certificate_reason=$message
        return 1
    fi
}

off_loopback_certificate_pin() {
    certificate_check 'The candidate Caddy upstream and the same owned private listener must be observed in all three phases without an added published port.' '
        . as $run | (.network_id | text) and (.caddy_id | text)
        and all([.initial, .rotated, .restored][];
            .listener == $run.initial.listener
            and (.listener | listener($run.network_id; $run.caddy_id))
            and .join.network_id == $run.network_id
            and .join.origin == (.listener | origin))' &&
    certificate_check 'The three attempts must use fresh enrollment tokens wrapped for the same listener and trusted leaf key.' '
        . as $run | (.initial.join.pin_sha256 | digest)
        and all([.initial, .rotated, .restored][];
            (.join.token_id | text) and .join.pin_sha256 == $run.initial.join.pin_sha256)
        and ([.initial.join.token_id, .rotated.join.token_id, .restored.join.token_id] | unique | length == 3)' &&
    certificate_check 'The initial matching leaf must be observed before a successful Join and authenticated revision 2 Open for that machine.' '
        .initial | phase and .leaf.spki_sha256 == .join.pin_sha256 and matching_control' &&
    certificate_check 'The listener must serve a valid leaf with a different key before the rejected Join begins.' '
        . as $run | .rotated | phase and $run.initial.completed_step < .leaf.observed_step
        and .leaf.certificate_sha256 != $run.initial.leaf.certificate_sha256
        and .leaf.spki_sha256 != .join.pin_sha256' &&
    # The relay records closed even if an application call forwards no frames.
    certificate_check 'The wrong-key client must report certificate_key_mismatch with zero forwarded application calls.' '
        .rotated | (.join.status | positive) and .join.refusal == "certificate_key_mismatch"
        and .relay == []' &&
    certificate_check 'Restoring the trusted key must precede a fresh successful Join and authenticated revision 2 Open for its new machine.' '
        . as $run | .restored | phase and $run.rotated.completed_step < .leaf.observed_step
        and .leaf.spki_sha256 == .join.pin_sha256 and .join.machine_id != $run.initial.join.machine_id
        and matching_control'
}

certificate_collect() {
    # This is a stub of the test writer. An empty candidate selects the synthetic
    # fixture. Its rotated leaf is unobserved until the collector is built.
    if [[ -z $1 ]]; then
        certificate_offline_observation | jq -c 'del(.rotated.leaf)'
    else
        printf '{}\n'
    fi
}

certificate_offline_observation() {
    # Synthetic identities and digests carry no enrollment or certificate material.
    jq -nc '
        def digest($digit): $digit * 64;
        def frame($connection; $stream; $machine; $direction; $frame):
            {connection_id: $connection, stream_id: $stream, machine_id: $machine,
             action: "forwarded", direction: $direction, frame: $frame};
        def matching_frames($join; $machine):
            [frame($join; "join-stream"; $machine; "up"; "join"),
             frame($join + 1; "open-stream"; $machine; "up"; "open") + {link_revision: 2},
             frame($join + 1; "open-stream"; $machine; "down"; "welcome") + {link_revision: 2}];
        def phase($step; $certificate; $spki; $token):
            {leaf: {observed_step: $step, valid_for_alias: true,
                    certificate_sha256: digest($certificate), spki_sha256: digest($spki)},
             join: {started_step: ($step + 1), token_id: $token,
                    origin: "https://fixture-relay:8443", pin_sha256: digest("1")},
             completed_step: ($step + 2)};
        def matching($connection; $machine):
            .join += {status: 0, machine_id: $machine} | .relay = matching_frames($connection; $machine);
        {id: "offline-listener", network_id: "offline-network", upstream_id: "offline-caddy",
         alias: "fixture-relay", port: 8443, added_published_ports: []} as $listener
        | {network_id: $listener.network_id, caddy_id: $listener.upstream_id,
           initial: (phase(1; "2"; "1"; "offline-first") | matching(1; "offline-machine-one")),
           rotated: (phase(4; "3"; "4"; "offline-wrong") |
               .join += {status: 1, refusal: "certificate_key_mismatch"} | .relay = []),
           restored: (phase(7; "2"; "1"; "offline-last") | matching(3; "offline-machine-two"))}
        | (.initial, .rotated, .restored) |=
            (.listener = $listener | .join.network_id = $listener.network_id)
    '
}

certificate_run_case() {
    local candidate=$1 certificate_reason='' certificate_observation=''
    printf '1..1\n'
    if certificate_observation=$(certificate_collect "$candidate" 2>/dev/null) &&
        off_loopback_certificate_pin; then
        printf 'ok 1 - off_loopback_certificate_pin\n'
        return 0
    fi
    printf 'not ok 1 - off_loopback_certificate_pin # %s\n' \
        "${certificate_reason:-The fixture did not collect the required observation.}"
    return 1
}

certificate_main() {
    if [[ $# != 1 || -z $1 || ( $1 == -* && $1 != --assertions ) ]]; then
        printf 'Usage: bash deploy/test/sprint4-certificate.sh <candidate-commit> or --assertions\n' >&2
        return 2
    fi
    if [[ $1 != --assertions && ${LGTM_SPRINT4_LIVE:-} != 1 ]]; then
        printf '1..0 # SKIP Set LGTM_SPRINT4_LIVE=1 on the owned candidate fixture to run the certificate gate.\n'
        return 0
    fi
    command -v jq >/dev/null 2>&1 || { printf 'The certificate assertions need jq.\n' >&2; return 2; }
    if [[ $1 == --assertions ]]; then
        certificate_run_case ''
    else
        certificate_run_case "$1"
    fi
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
    certificate_main "$@"
fi
