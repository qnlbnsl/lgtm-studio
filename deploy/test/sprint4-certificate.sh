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
    if [[ -z $1 ]]; then
        certificate_offline_observation
    else
        certificate_live "$1"
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

certificate_build() {
    # Use the candidate's generated JoinToken type for both sides of the rewrap.
    cat >"$work/helper.go" <<'GO' || return 1
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) == 5 {
		switch os.Args[1] {
		case "rewrap":
			if rewrap(os.Args[2], os.Args[3], os.Args[4]) {
				return
			}
		case "probe":
			if probe(os.Args[2], os.Args[3], os.Args[4]) {
				return
			}
		}
	}
	os.Exit(1)
}

func rewrap(origin, pinHex, destination string) bool {
	const prefix = "lgtm1_"
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 4097))
	if err != nil || len(input) > 4096 {
		return false
	}
	body, ok := strings.CutPrefix(strings.TrimSuffix(string(input), "\n"), prefix)
	if !ok {
		return false
	}
	encoding := base64.RawURLEncoding.Strict()
	raw, err := encoding.DecodeString(body)
	if err != nil || encoding.EncodeToString(raw) != body {
		return false
	}
	issued := new(linkv1.JoinToken)
	if proto.Unmarshal(raw, issued) != nil || len(issued.ProtoReflect().GetUnknown()) != 0 ||
		len(issued.GetSecret()) != sha256.Size || len(issued.GetServerKeySha256()) != sha256.Size ||
		!regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`).MatchString(issued.GetTokenId()) {
		return false
	}
	marshal := proto.MarshalOptions{Deterministic: true}
	canonical, err := marshal.Marshal(issued)
	if err != nil || !bytes.Equal(raw, canonical) {
		return false
	}
	pin, err := hex.DecodeString(pinHex)
	if err != nil || len(pin) != sha256.Size {
		return false
	}
	wrapped := proto.CloneOf(issued)
	wrapped.Address = origin
	wrapped.CertificateKeySha256 = pin
	raw, err = marshal.Marshal(wrapped)
	if err != nil {
		return false
	}
	check := new(linkv1.JoinToken)
	if proto.Unmarshal(raw, check) != nil || !proto.Equal(wrapped, check) ||
		check.GetTokenId() != issued.GetTokenId() || !bytes.Equal(check.GetSecret(), issued.GetSecret()) ||
		!bytes.Equal(check.GetServerKeySha256(), issued.GetServerKeySha256()) {
		return false
	}
	if os.WriteFile(destination, []byte(prefix+encoding.EncodeToString(raw)+"\n"), 0600) != nil {
		return false
	}
	_, err = fmt.Fprintln(os.Stdout, issued.GetTokenId())
	return err == nil
}

func probe(alias, port, expectedIP string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dialer := tls.Dialer{Config: &tls.Config{
		ServerName: alias, MinVersion: tls.VersionTLS12,
		// The fixture's self-signed leaf is checked below against its name and time.
		InsecureSkipVerify: true,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(alias, port))
	if err != nil {
		return false
	}
	defer conn.Close()
	peer, ok := conn.RemoteAddr().(*net.TCPAddr)
	leaves := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if !ok || !peer.IP.Equal(net.ParseIP(expectedIP)) || len(leaves) != 1 {
		return false
	}
	leaf := leaves[0]
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: alias, Roots: roots}); err != nil {
		return false
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"valid_for_alias":    true,
		"certificate_sha256": fmt.Sprintf("%x", sha256.Sum256(leaf.Raw)),
		"spki_sha256":        fmt.Sprintf("%x", sha256.Sum256(leaf.RawSubjectPublicKeyInfo)),
	}) == nil
}
GO
    (
        cd -- "$S4_CANDIDATE_ROOT" &&
            export CGO_ENABLED=0 GOOS=linux GOARCH="$architecture" &&
            go build -mod=readonly -trimpath -buildvcs=false -o "$work/bin/lgtm-client" ./src/client/cmd/lgtm-client &&
            go build -mod=readonly -trimpath -buildvcs=false -o "$work/bin/sprint4relay" ./deploy/test/sprint4relay &&
            go build -mod=readonly -trimpath -buildvcs=false -o "$work/bin/certificate-helper" "$work/helper.go"
    ) &&
        chmod 0755 "$work/bin" &&
        chmod 0555 "$work/bin/lgtm-client" "$work/bin/sprint4relay" "$work/bin/certificate-helper" &&
        s4_source_clean
}

certificate_make_pair() {
    local name=$1
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 1 \
        -subj "/CN=$alias" -addext "subjectAltName=DNS:$alias" -addext 'extendedKeyUsage=serverAuth' \
        -out "$work/$name.crt" -keyout "$work/$name.key" >/dev/null 2>&1 &&
        cat -- "$work/$name.crt" "$work/$name.key" >"$work/$name.pem"
}

certificate_pair_on_listener() {
    cp -- "$work/$1.pem" "$work/relay/next.pem" &&
        mv -f -- "$work/relay/next.pem" "$work/relay/leaf.pem"
}

certificate_socket() {
    # Read the listening socket inode through the relay process's own descriptors.
    "$s4_docker" exec "$relay" /bin/sh -c '
        set -- $(awk -v port=":$1" '\''$2 ~ (port "$") && $4 == "0A" {print $2, $10}'\'' /proc/net/tcp)
        [ "$#" -eq 2 ] || exit 1
        for fd in /proc/1/fd/*; do
            [ "$(readlink "$fd")" = "socket:[$2]" ] || continue
            active=$(awk -v address="$1" '\''
                $2 == address && $4 != "0A" && $4 != "06" {count++}
                END {print count+0}
            '\'' /proc/net/tcp) || exit 1
            printf "%s %s\n" "$2" "$active"
            exit 0
        done
        exit 1
    ' sh "$(printf '%04X' "$port")"
}

certificate_listener() {
    local caddy_model relay_model socket
    sprint4_fixture_ready &&
        caddy_model=$("$s4_docker" container inspect "$caddy") &&
        s4_container web <<<"$caddy_model" &&
        relay_model=$("$s4_docker" container inspect "$relay") &&
        socket=$(certificate_socket) || return 1
    jq -ce --argjson caddy "$caddy_model" --arg name "$network_name" --arg alias "$alias" \
        --arg ip "$listen_ip" --arg run "$s4_run" --argjson port "$port" \
        --arg inode "${socket%% *}" '
        .[0] as $relay | $caddy[0] as $web | $web.NetworkSettings.Networks[$name] as $endpoint
        | select($web.State.Running and $relay.State.Running
            and ($relay.State.Restarting | not) and $relay.RestartCount == 0
            and $relay.Config.Labels["lgtm.sprint4.certificate"] == $run
            and $relay.HostConfig.NetworkMode == ("container:" + $web.Id)
            and $endpoint.IPAddress == $ip and ($endpoint.Aliases | index($alias)) != null
            and ($relay.HostConfig.PortBindings // {} | length) == 0
            and ([$relay.NetworkSettings.Ports[]? | .[]?] | length) == 0)
        | {id: ([$relay.Id, $relay.State.StartedAt, $inode] | join("/")),
           network_id: $endpoint.NetworkID, upstream_id: $web.Id, $alias, $port,
           added_published_ports: []}
    ' <<<"$relay_model"
}

certificate_client_network() {
    local model network
    model=$("$s4_docker" container inspect "$client") &&
        network=$("$s4_docker" network inspect "$network_id") || return 1
    jq -er --argjson network "$network" --arg name "$network_name" --arg run "$s4_run" \
        --arg web "$caddy" '
        .[0] as $client | $network[0] as $net
        | select($client.State.Running and $net.Internal
            and $client.Config.Labels["lgtm.sprint4.certificate"] == $run
            and $net.Labels["lgtm.sprint4.certificate"] == $run
            and ($client.HostConfig.PortBindings // {} | length) == 0
            and ([$client.NetworkSettings.Ports[]? | .[]?] | length) == 0
            and ($client.NetworkSettings.Networks | keys) == [$name]
            and ($net.Containers | keys) == ([$web, $client.Id] | sort)
            and $client.NetworkSettings.Networks[$name].NetworkID == $net.Id)
        | $net.Id
    ' <<<"$model"
}

certificate_probe() {
    local pair=$1 expected_leaf deadline=$((SECONDS + 20)) leaf
    expected_leaf=$(openssl x509 -in "$work/$pair.crt" -outform DER | openssl dgst -sha256 -r) || return 1
    while ((SECONDS < deadline)); do
        if leaf=$("$s4_docker" exec "$client" /proof/certificate-helper probe "$alias" "$port" "$listen_ip") &&
            jq -e --arg expected "${expected_leaf%% *}" '.certificate_sha256 == $expected' \
                <<<"$leaf" >/dev/null; then
            printf '%s\n' "$leaf"
            return 0
        fi
        sleep 0.2
    done
    return 1
}

certificate_window() {
    jq -cs --argjson offset "$offset" '
        if length < $offset then error("The relay window was truncated.") end
        | .[$offset:] | map({connection_id, stream_id, machine_id, link_revision, action, direction, frame})
    ' "$work/relay/observations.jsonl"
}

certificate_wait_welcome() {
    local machine=$1 deadline=$((SECONDS + 30)) window
    while ((SECONDS < deadline)); do
        if window=$(certificate_window) &&
            jq -e --arg machine "$machine" '
                any(.[]; .action == "forwarded" and .direction == "down" and .frame == "welcome"
                    and .machine_id == $machine and .link_revision == 2)
            ' <<<"$window" >/dev/null; then
            return 0
        fi
        sleep 0.2
    done
    return 1
}

certificate_drain() {
    local window previous='' socket deadline=$((SECONDS + 20))
    while ((SECONDS < deadline)); do
        if socket=$(certificate_socket) && [[ ${socket#* } == 0 ]] &&
            window=$(certificate_window) &&
            jq -e 'all(group_by(.stream_id)[]; last.action == "closed")' <<<"$window" >/dev/null; then
            if [[ $window == "$previous" ]]; then
                printf '%s\n' "$window"
                return 0
            fi
            previous=$window
        else
            previous=''
        fi
        sleep 0.5
    done
    return 1
}

certificate_phase() {
    local phase=$1 pair=$2 step=$3 listener leaf client_network token_id status refusal='' machine='' window
    local -a join_status
    client=$("$s4_docker" run -d --pull=never --name "$resource-$phase" --network "$network_id" \
        "${container_options[@]}" --user 10003:10003 --env HOME=/state --env LGTM_CLIENT_STATE_DIR=/state \
        --tmpfs /state:rw,noexec,nosuid,nodev,mode=0700,uid=10003,gid=10003 \
        --entrypoint /bin/sleep "$image" 180) &&
        client_network=$(certificate_client_network) &&
        listener=$(certificate_listener) &&
        jq -e --arg network "$client_network" '.network_id == $network' <<<"$listener" >/dev/null &&
        [[ $(certificate_window) == '[]' ]] &&
        certificate_pair_on_listener "$pair" &&
        leaf=$(certificate_probe "$pair") &&
        jq -e --arg phase "$phase" --arg pin "$pin" '
            if $phase == "rotated" then .spki_sha256 != $pin else .spki_sha256 == $pin end
        ' <<<"$leaf" >/dev/null || return 1

    token_id=$(sprint4_fixture_run docker compose exec -T server lgtm-server join-token |
        "$s4_docker" exec -i "$client" /proof/certificate-helper rewrap "$origin" "$pin" /state/token) ||
        return 1
    if timeout --kill-after=3s 45s "$s4_docker" exec "$client" /bin/sh -c '
        exec /proof/lgtm-client join "$1" - </state/token
    ' sh "$origin" 2>&1 | jq -Rr '
        fromjson? | select(.msg == "join_refused" and .code == "certificate_key_mismatch") | .code
    ' >"$work/refusal"; then
        join_status=("${PIPESTATUS[@]}")
    else
        join_status=("${PIPESTATUS[@]}")
    fi
    status=${join_status[0]}
    [[ ${join_status[1]} == 0 ]] && ((status < 124)) || return 1
    refusal=$(<"$work/refusal")
    "$s4_docker" exec "$client" rm -f /state/token >/dev/null || return 1
    if ((status == 0)); then
        machine=$("$s4_docker" exec "$client" cat /state/identity.json |
            jq -er '.machineId | select(test("^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$"))') || return 1
        "$s4_docker" exec -d "$client" /bin/sh -c '
            printf "%s\n" "$$" >/state/run.pid || exit 1
            exec /proof/lgtm-client run >/dev/null 2>&1
        ' && certificate_wait_welcome "$machine" &&
            timeout 10s "$s4_docker" exec "$client" /bin/sh -c '
                pid=$(cat /state/run.pid) && kill -TERM "$pid" || exit 1
                while kill -0 "$pid" 2>/dev/null; do sleep 0.1; done
            ' || return 1
    fi
    window=$(certificate_drain) &&
        "$s4_docker" container rm -f "$client" >/dev/null &&
        [[ $(certificate_window) == "$window" ]] &&
        [[ $(certificate_listener) == "$listener" ]] || return 1

    jq -nc --argjson listener "$listener" --argjson leaf "$leaf" --argjson relay "$window" \
        --arg network_id "$client_network" --arg origin "$origin" --arg token_id "$token_id" \
        --arg pin_sha256 "$pin" --arg machine_id "$machine" --arg refusal "$refusal" \
        --argjson status "$status" --argjson step "$step" '
        def count_requests:
            map(select(.action == "forwarded" and .direction == "up"
                and (.frame == "join" or .frame == "open"))) | length;
        { $listener, leaf: ($leaf + {observed_step: $step}), $relay, completed_step: ($step + 2),
          join: ({started_step: ($step + 1), $network_id, $origin, $token_id, $pin_sha256, $status}
                 + if $status == 0 then {$machine_id} else {$refusal} end) }
        + {forwarded_requests_by_connection: (
            $relay | group_by(.connection_id) | map({
                connection_id: first.connection_id,
                forwarded_requests: count_requests
            })
        ), forwarded_requests: ($relay | count_requests)}
    '
}

certificate_close() {
    local containers networks id status=0
    if [[ -n ${resource:-} ]]; then
        if [[ $(s4_daemon) != "$s4_daemon_id" ]]; then
            return 1
        fi
        containers=$("$s4_docker" container ls --all --no-trunc --quiet \
            --filter "label=lgtm.sprint4.certificate=$s4_run") || return 1
        networks=$("$s4_docker" network ls --no-trunc --quiet \
            --filter "label=lgtm.sprint4.certificate=$s4_run") || return 1
        while IFS= read -r id; do
            [[ -z $id ]] || "$s4_docker" container rm -f "$id" >/dev/null || status=1
        done <<<"$containers"
    fi
    sprint4_fixture_close || status=1
    while IFS= read -r id; do
        [[ -z $id ]] || "$s4_docker" network rm "$id" >/dev/null || status=1
    done <<<"${networks:-}"
    return "$status"
}

certificate_live() (
    local candidate=$1 root work resource='' network_name network_id caddy image architecture listen_ip relay
    local alias=fixture-relay port=8443 origin=https://fixture-relay:8443 pin
    local client offset=0 rotated_count initial rotated restored observation evidence
    local -a container_options
    root=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P) &&
        source "$root/deploy/test/sprint4-fixture.sh" &&
        sprint4_fixture_open "$root" "$candidate" || return 1
    trap 'status=$?; certificate_close || status=1; exit "$status"' EXIT
    s4_git -C "$S4_CANDIDATE_ROOT" merge-base --is-ancestor 89506b7c "$s4_commit" &&
        sprint4_select 4 &&
        sprint4_fixture_run bash "$S4_CANDIDATE_ROOT/deploy/up.sh" >/dev/null 2>&1 &&
        sprint4_fixture_ready || return 1
    caddy=$(awk -v name="$s4_project-web-1" '$1 == "container" && $2 == name {print $3}' "$s4_dir/objects") &&
        [[ $caddy =~ ^[0-9a-f]{64}$ ]] &&
        image=$("$s4_docker" container inspect --format '{{.Image}}' "$caddy") &&
        architecture=$("$s4_docker" image inspect --format '{{.Architecture}}' "$image") || return 1

    work=$s4_dir/certificate
    mkdir -p -- "$work/bin" "$work/relay" &&
        certificate_build &&
        certificate_make_pair trusted &&
        certificate_make_pair wrong &&
        certificate_pair_on_listener trusted || return 1
    pin=$(openssl x509 -in "$work/trusted.crt" -pubkey -noout |
        openssl pkey -pubin -outform DER | openssl dgst -sha256 -r) || return 1
    pin=${pin%% *}

    resource=lgtm-s4cert-$s4_run
    network_name=$resource-net
    container_options=(--label "lgtm.sprint4.certificate=$s4_run" --log-driver none
        --read-only --no-healthcheck --cap-drop ALL --security-opt no-new-privileges
        --mount "type=bind,source=$work/bin,target=/proof,readonly")
    network_id=$("$s4_docker" network create --driver bridge --internal \
        --label "lgtm.sprint4.certificate=$s4_run" "$network_name") &&
        "$s4_docker" network connect --alias "$alias" "$network_id" "$caddy" &&
        listen_ip=$("$s4_docker" container inspect "$caddy" | jq -er --arg name "$network_name" \
            --arg id "$network_id" '.[0].NetworkSettings.Networks[$name] | select(.NetworkID == $id) | .IPAddress') &&
        [[ -n $listen_ip ]] || return 1
    # Sharing Caddy's namespace keeps its upstream Host on the deployed loopback route.
    relay=$("$s4_docker" run -d --pull=never --name "$resource-relay" --network "container:$caddy" \
        "${container_options[@]}" --user "$(id -u):$(id -g)" \
        --mount "type=bind,source=$work/relay,target=/relay" --entrypoint /proof/sprint4relay "$image" \
        --listen "$listen_ip:$port" --upstream http://127.0.0.1:8080 --control-dir /relay \
        --tls-cert /relay/leaf.pem --tls-key /relay/leaf.pem) || return 1
    # Only the listener's startup is polled here. The leaf probe runs from each fresh client.
    local deadline=$((SECONDS + 20))
    until certificate_socket >/dev/null 2>&1; do
        ((SECONDS < deadline)) || return 1
        sleep 0.2
    done
    initial=$(certificate_phase initial trusted 1) &&
        offset=$(jq -er '.relay | length' <<<"$initial") &&
        rotated=$(certificate_phase rotated wrong 4) &&
        rotated_count=$(jq -er '.relay | length' <<<"$rotated") &&
        offset=$((offset + rotated_count)) &&
        restored=$(certificate_phase restored trusted 7) &&
        s4_source_clean || return 1
    observation=$(jq -nc --arg candidate_commit "$s4_commit" --arg network_id "$network_id" \
        --arg caddy_id "$caddy" --argjson initial "$initial" --argjson rotated "$rotated" \
        --argjson restored "$restored" '{$candidate_commit, $network_id, $caddy_id, $initial, $rotated, $restored}') ||
        return 1

    certificate_close || return 1
    trap - EXIT
    # Keep only the sanitized record after the owned fixture is removed.
    evidence=$(mktemp "${LGTM_SPRINT4_CERTIFICATE_EVIDENCE_DIR:-$PWD}/sprint4-certificate-evidence.XXXXXXXX") &&
        printf '%s\n' "$observation" >"$evidence" &&
        printf '%s\n' "$observation"
)

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
    certificate_main "$@"
fi
