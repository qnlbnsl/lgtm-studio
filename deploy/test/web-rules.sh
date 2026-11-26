#!/usr/bin/env bash
# This suite runs the web rules with a static site and an upstream that records requests.
set -u -o pipefail
export LC_ALL=C

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
caddy=caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0
work=$(mktemp -d) || exit 1
project=lgtm-s2w-${work##*.}
project=${project,,}
web=$project-web
server=$project-server
network=$project-net
security=(--user 10003:10003 --read-only --cap-drop ALL --security-opt no-new-privileges:true
    --tmpfs /tmp -e XDG_DATA_HOME=/tmp -e XDG_CONFIG_HOME=/tmp)
passed=0
failed=0
port=''
status=''

cleanup() {
    docker rm -fv "$web" "$server" >/dev/null 2>&1 || true
    docker network rm "$network" >/dev/null 2>&1 || true
    rm -rf -- "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

check() {
    local name=$1
    if "$name"; then
        printf 'PASS [%s]\n' "$name"
        ((passed += 1))
    else
        printf 'FAIL [%s]\n' "$name"
        ((failed += 1))
    fi
}

request() {
    local method=$1 path=$2
    shift 2
    status=$(curl --noproxy '*' --silent --show-error --max-time 3 --request "$method" \
        --dump-header "$work/headers" --output "$work/body" --write-out '%{http_code}' \
        "http://127.0.0.1:$port$path" "$@")
}

setup() {
    mkdir "$work/site" "$work/site/assets" || return 1
    printf '%s\n' '<main id="app"></main><script src="/assets/a.js"></script>' >"$work/site/index.html"
    printf '%s\n' 'console.log("fixture");' >"$work/site/assets/a.js"
    cat >"$work/upstream.Caddyfile" <<'UPSTREAM'
{
	admin off
	auto_https off
	persist_config off
	servers {
		protocols h1 h2c
	}
}

http://:8080 {
	log {
		output stdout
		format json
	}
	handle /lgtm.v1.SessionService/SignOut {
		header Content-Type application/json
		respond `{"code":"unauthenticated","message":"Sign in first."}` 401
	}
	handle {
		respond "{http.request.proto} {method} {path} host={http.request.host} xfp={header.X-Forwarded-Proto}"
	}
}
UPSTREAM
    docker network create "$network" >/dev/null &&
        docker run -d --name "$server" --network "$network" --network-alias server "${security[@]}" \
            -v "$work/upstream.Caddyfile:/etc/caddy/Caddyfile:ro" "$caddy" >/dev/null &&
        docker run -d --name "$web" --network "$network" --publish 127.0.0.1::8080 "${security[@]}" \
            -v "$root/deploy/Caddyfile:/etc/caddy/Caddyfile:ro" -v "$work/site:/srv/site:ro" "$caddy" >/dev/null ||
        return 1
    port=$(docker port "$web" 8080/tcp) || return 1
    [[ $port == 127.0.0.1:* ]] || return 1
    port=${port##*:}
    local deadline=$((SECONDS + 15))
    until request GET / 2>/dev/null &&
        docker exec "$server" wget -q -T 2 -O /dev/null http://127.0.0.1:8080/probe-ready; do
        ((SECONDS < deadline)) || return 1
        sleep 0.2
    done
}

# A known request has been logged before the log can prove the absence of another.
upstream_saw_no_paths() {
    local known=$1
    shift
    local paths deadline=$((SECONDS + 4))
    while :; do
        paths=$(docker logs "$server" 2>/dev/null |
            jq -r 'select((.logger // "") | startswith("http.log.access")) | .request.uri') || return 1
        if grep -Fxq -- "$known" <<<"$paths"; then
            break
        fi
        ((SECONDS < deadline)) || return 1
        sleep 0.1
    done
    local path
    for path; do
        if grep -Fxq -- "$path" <<<"$paths"; then
            return 1
        fi
    done
}

rules_serve_the_website() {
    request GET / && [[ $status == 200 ]] && grep -Fq 'id="app"' "$work/body" &&
        request GET /assets/a.js && [[ $status == 200 ]] &&
        grep -Eiq '^Content-Type:[[:blank:]]*text/javascript([;[:space:]]|$)' "$work/headers" &&
        request GET /no/such/file && [[ $status == 404 ]]
}

rules_pass_contract_paths_on() {
    local path
    for path in /lgtm.v1.SessionService/GetServer /lgtm.link.v1.LinkService/Join; do
        request POST "$path" && [[ $status == 200 ]] &&
            [[ $(cat "$work/body") == "HTTP/2.0 POST $path "* ]] || return 1
    done
    request POST /lgtm.v2.X && [[ $status == 404 ]] &&
        request GET /frames/x && [[ $status == 404 ]] &&
        upstream_saw_no_paths /lgtm.link.v1.LinkService/Join /lgtm.v2.X /frames/x
}

rules_keep_status_and_body() {
    request POST /lgtm.v1.SessionService/SignOut && [[ $status == 401 ]] &&
        grep -Fixq $'Content-Type: application/json\r' "$work/headers" &&
        printf '%s' '{"code":"unauthenticated","message":"Sign in first."}' | cmp -s - "$work/body"
}

rules_keep_host_and_set_forwarded_proto() {
    request POST /lgtm.v1.SessionService/GetServer --header 'Host: 127.0.0.1' \
        --header 'X-Forwarded-Proto: https' && [[ $status == 200 ]] &&
        [[ $(cat "$work/body") == *' host=127.0.0.1 xfp=http' ]]
}

rules_refuse_other_host_names() {
    request GET / --header 'Host: lgtm.test:8787' && [[ $status == 421 ]] &&
        request POST /lgtm.v1.SessionService/SignIn --header 'Host: lgtm.test:8787' && [[ $status == 421 ]] &&
        upstream_saw_no_paths /probe-ready /lgtm.v1.SessionService/SignIn / || return 1
    local host
    for host in localhost '[::1]'; do
        request GET / --header "Host: $host:8787" && [[ $status == 200 ]] &&
            grep -Fq 'id="app"' "$work/body" || return 1
    done
}

rules_answer_health_inside_only() {
    docker exec "$web" wget -q -T 3 -O /dev/null http://127.0.0.1:8081/healthz &&
        docker exec "$server" wget -q -T 3 -O /dev/null --header 'Host: localhost' "http://$web:8080/" ||
        return 1
    local output
    output=$(docker exec "$server" wget -T 3 -O /dev/null "http://$web:8081/healthz" 2>&1) && return 1
    [[ $output == *'Connection refused'* ]]
}

if ! setup; then
    printf 'FAIL [setup] The web rules or the upstream did not start.\n' >&2
    docker logs "$web" >&2 || true
    docker logs "$server" >&2 || true
    printf '0 passed, 1 failed\n'
    exit 1
fi

check rules_serve_the_website
check rules_pass_contract_paths_on
check rules_keep_status_and_body
check rules_keep_host_and_set_forwarded_proto
check rules_refuse_other_host_names
check rules_answer_health_inside_only
printf '%d passed, %d failed\n' "$passed" "$failed"
((failed == 0))
