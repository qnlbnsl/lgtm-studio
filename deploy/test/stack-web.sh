#!/usr/bin/env bash
# This suite checks the Compose model without building the server or website.
set -u -o pipefail
export LC_ALL=C

source "$(dirname -- "${BASH_SOURCE[0]}")/web-testlib.sh"
web_passed=0 web_failed=0
web_fixture_create || exit 1
trap web_fixture_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

config() { docker compose --profile '*' config --format json; }

with_port_only() {
    printf 'LGTM_WEB_PORT=%s\n' "$1" >"$COMPOSE_ENV_FILES"
}

web_publishes_on_loopback_only() {
    local port
    for port in 8787 18787; do
        if [[ $port == 8787 ]]; then
            : >"$COMPOSE_ENV_FILES"
        else
            with_port_only "$port"
        fi
        config | jq -e --arg port "$port" '
            .services.web.ports == [{mode:"ingress", host_ip:"127.0.0.1", published:$port,
                target:8080, protocol:"tcp"}]
            and (.services | del(.web) | all(.[]; ((.ports // []) == [])))
        ' >/dev/null || return 1
    done
}

server_has_no_route_out() {
    config | jq -e '
        ((.services.server.networks // {}) | keys) == ["app", "data", "vault"]
        and ((.services.web.networks // {}) | keys) == ["app", "edge"]
        and ([.networks.app, .networks.data, .networks.vault] | all(.[]; .internal == true))
        and (.networks.edge | type == "object" and .internal != true and (.driver // "bridge") == "bridge")
    ' >/dev/null || return 1

    cat >"$web_folder/network.yaml" <<EOF_NETWORK
services:
  target:
    image: $web_caddy
    entrypoint: ["caddy", "respond", "--listen", ":9000", "ok"]
    networks: [app]
  probe:
    image: $web_caddy
    entrypoint: ["/bin/sh"]
    networks: [app, data, vault]
EOF_NETWORK
    web_fixture_override "$web_folder/network.yaml" &&
        config | web_fixture_owns_model || return 1
    docker compose run -d --rm --no-deps --use-aliases --name "$web_project-target" target \
        >"$web_folder/network.log" 2>&1 || { cat "$web_folder/network.log" >&2; return 1; }
    timeout 35 docker compose run --rm --no-deps -T --name "$web_project-probe" probe -ec '
        for attempt in 1 2 3 4 5; do
            body=$(wget -T 3 -qO- http://target:9000/) && [ "$body" = ok ] && break
            sleep 1
        done
        [ "$body" = ok ]
        status=0
        timeout 10 wget -T 3 -O /dev/null http://example.com/ || status=$?
        [ "$status" -eq 1 ]
    ' >>"$web_folder/network.log" 2>&1 || { cat "$web_folder/network.log" >&2; return 1; }
}

server_settings_are_named() {
    printf 'LGTM_WEB_PORT=18787\nLGTM_WEB_PASSWORD=%s\n' "$web_password" >"$COMPOSE_ENV_FILES"
    config | WEB_TEST_PASSWORD=$web_password jq -e '
        .services.server.environment == {
            LGTM_WEB_PASSWORD:env.WEB_TEST_PASSWORD, LGTM_LISTEN_ADDR:":8080",
            LGTM_PUBLIC_URL:"http://127.0.0.1:18787", LGTM_POSTGRES_HOST:"postgres",
            LGTM_POSTGRES_PORT:"5432", LGTM_POSTGRES_DATABASE:"lgtm_app", LGTM_POSTGRES_USER:"lgtm_app",
            LGTM_POSTGRES_PASSWORD_FILE:"/run/lgtm/postgres.password", LGTM_ALLOW_PRIVATE_STACK:"true",
            LGTM_OPENBAO_ADDR:"http://openbao:8200", LGTM_OPENBAO_CREDENTIAL_FILE:"/run/lgtm/openbao.json"
        }
    ' >/dev/null || return 1
    printf 'LGTM_WEB_PASSWORD=%s\n' "$web_password" >"$COMPOSE_ENV_FILES"
    config | jq -e '.services.server.environment.LGTM_PUBLIC_URL == "http://127.0.0.1:8787"' >/dev/null
}

password_follows_the_checks() {
    local original override shell_password
    printf 'LGTM_WEB_PASSWORD=%s\n' "$web_password" >"$COMPOSE_ENV_FILES"
    original=$(config) || return 1
    shell_password=$(web_fixture_random 16) || return 1
    LGTM_WEB_PASSWORD=$shell_password config |
        WEB_TEST_PASSWORD=$shell_password jq -e '.services.server.environment.LGTM_WEB_PASSWORD == env.WEB_TEST_PASSWORD' \
        >/dev/null || return 1

    # Read the same override that the frozen rollout driver feeds to Compose.
    override=$web_folder/check-c2.yaml
    awk '
        /^check_c2\(\)/ { in_check = 1 }
        in_check && /<<.YAML./ { copying = 1; next }
        copying && /^YAML$/ { exit }
        copying { print }
    ' "$web_root/deploy/test/checks.sh" >"$override" || return 1
    [[ -s $override ]] && web_fixture_override "$override" || return 1
    config | jq -e --slurpfile original <(printf '%s\n' "$original") '
        .services.server.environment == ($original[0].services.server.environment | del(.LGTM_WEB_PASSWORD))
        and ($original[0].services.server.environment | has("LGTM_WEB_PASSWORD"))
    ' >/dev/null
}

services_are_unprivileged() {
    config | jq -e '
        .services as $s |
        $s.server.user == "10001:10001" and $s.web.user == "10003:10003"
        and ([$s.server, $s.web] | all(.[]; .read_only == true and .cap_drop == ["ALL"]
            and (.security_opt | index("no-new-privileges:true") != null)
            and (.tmpfs | index("/tmp") != null) and .restart == "unless-stopped"
            and (has("cap_add") or has("privileged") or has("pid") | not)))
    ' >/dev/null
}

server_mounts_are_read_only() {
    config | jq -e --arg state "$LGTM_STACK_STATE_DIR" '
        ((.services.server.volumes // []) | sort_by(.target)) ==
        [{type:"bind", source:($state + "/server/openbao.json"), target:"/run/lgtm/openbao.json",
          read_only:true, bind:{create_host_path:false}},
         {type:"bind", source:($state + "/server/postgres-app.password"), target:"/run/lgtm/postgres.password",
          read_only:true, bind:{create_host_path:false}}]
        and ((.services.web.volumes // []) == []) and ((.services.web.environment // {}) == {})
    ' >/dev/null
}

server_waits_for_database_and_vault() {
    config | jq -e '
        def dependency_conditions: (. // {}) | map_values(.condition);
        .services as $s |
        ($s.server.depends_on | dependency_conditions) == {postgres:"service_healthy", openbao:"service_healthy"}
        and ($s.web.depends_on | dependency_conditions) == {server:"service_healthy"}
        and $s.server.healthcheck == {
            test:["CMD", "/usr/local/bin/lgtm-server", "health"], start_period:"8m0s",
            start_interval:"2s", interval:"10s", timeout:"3s", retries:3}
        and $s.web.healthcheck == {
            test:["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8081/healthz"], start_period:"30s",
            start_interval:"1s", interval:"10s", timeout:"3s", retries:3}
    ' >/dev/null
}

images_are_built_from_the_pinned_files() {
    local tag
    for tag in local "$web_project"; do
        if [[ $tag == local ]]; then
            unset LGTM_IMAGE_TAG
        else
            export LGTM_IMAGE_TAG=$tag
        fi
        config | jq -e --arg root "$(git -C "$web_root" rev-parse --show-toplevel)" --arg tag "$tag" '
            .services as $s |
            ($s.server.build | [.context, .dockerfile, .target]) == [$root, "deploy/server.Dockerfile", "runtime"]
            and ($s.web.build | [.context, .dockerfile, .target]) == [$root, "deploy/website.Dockerfile", "runtime"]
            and $s.server.image == ("lgtm-server:" + $tag) and $s.web.image == ("lgtm-web:" + $tag)
            and ($s | del(.server, .web) | all(.[]; .image | test("@sha256:[0-9a-f]{64}$")))
        ' >/dev/null || return 1
    done
}

example_settings_list_every_setting() {
    awk '
        /^[[:space:]]*$/ { comment = 0; next }
        /^[[:space:]]*#[[:space:]]*[^[:space:]]/ { comment = 1; next }
        {
            if (!comment || ($0 != "LGTM_WEB_PASSWORD=" && $0 != "LGTM_WEB_PORT=8787")) bad = 1
            count[$0]++
            comment = 0
        }
        END { exit (bad || count["LGTM_WEB_PASSWORD="] != 1 || count["LGTM_WEB_PORT=8787"] != 1) }
    ' "$web_root/deploy/example.env"
}

# The tests change only their fixture settings, and restore them between assertions.
cp -- "$COMPOSE_ENV_FILES" "$web_folder/original.env" || exit 1
for assertion in web_publishes_on_loopback_only server_has_no_route_out server_settings_are_named \
    password_follows_the_checks services_are_unprivileged server_mounts_are_read_only \
    server_waits_for_database_and_vault images_are_built_from_the_pinned_files example_settings_list_every_setting; do
    cp -- "$web_folder/original.env" "$COMPOSE_ENV_FILES" || exit 1
    web_check "$assertion"
done
web_finish
