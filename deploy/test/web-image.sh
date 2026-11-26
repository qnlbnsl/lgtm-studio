#!/usr/bin/env bash
# This suite builds the site and runtime images from a copy with decoy credentials.
set -u -o pipefail
export LC_ALL=C

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
node=node:26.13.0-alpine@sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216
caddy=caddy:2.11.9-alpine@sha256:003a97808e781aa9f7e75cc2f43d34d461099031faf5e20faefb9f61126836f0
work=$(mktemp -d) || exit 1
project=lgtm-s2w-${work##*.}
project=${project,,}
site_image=$project:site
web_image=$project:web
snapshot=$work/checkout
export BUILDX_CONFIG=$work/buildx
dockerfile=deploy/website.Dockerfile
sentinel=''
built=0
passed=0
failed=0
site_url=''

cleanup() {
    docker rm -fv "$project-site" "$project-hardening" "$project-scan" "$project-rules" "$project-tools" \
        >/dev/null 2>&1 || true
    docker image rm "$web_image" "$site_image" >/dev/null 2>&1 || true
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

marker() { od -An -N16 -tx1 /dev/urandom | tr -d ' \n'; }

build_images() {
    mkdir "$snapshot" || return 1
    (
        cd "$root" || exit 1
        git ls-files -z |
            while IFS= read -r -d '' file; do
                if [[ -e $file || -L $file ]]; then
                    printf '%s\0' "$file"
                fi
            done |
            tar -c --ignore-failed-read --no-recursion --null -T - |
            tar -xf - -C "$snapshot"
    ) || return 1
    # These files may be new and are copied even before the orchestrator stages them.
    cp -- "$root/$dockerfile" "$root/$dockerfile.dockerignore" "$root/deploy/Caddyfile" "$snapshot/deploy/" ||
        return 1
    # The decoys are always created inside the temporary copy.
    mkdir -p "$snapshot/deploy/.state" "$snapshot/src/frontend/.cert" || return 1
    local path value suffix
    suffix=$(marker) || return 1
    sentinel=sentinel-$suffix
    for path in "deploy/.state/$sentinel" deploy/.env src/frontend/.cert/key.pem; do
        value=$(marker) || return 1
        printf '%s\n' "$value" >>"$work/markers"
        printf '%s\n' "$value" >"$snapshot/$path"
    done

    docker build --target site --tag "$site_image" --file "$snapshot/$dockerfile" "$snapshot" >"$work/site.log" 2>&1 &&
        docker build --tag "$web_image" --file "$snapshot/$dockerfile" "$snapshot" >"$work/build.log" 2>&1 &&
        docker image inspect "$site_image" "$web_image" |
        jq -e 'all(.[]; (.Config.Volumes // {}) | length == 0)' >/dev/null
}

request() {
    curl --noproxy '*' --silent --show-error --max-time 4 --output "$work/body" \
        --write-out '%{http_code}' "$site_url$1"
}

start_site() {
    local container=$1
    shift
    local binding deadline=$((SECONDS + 15))
    docker run -d --name "$container" --publish 127.0.0.1::8080 "$@" "$web_image" >/dev/null || return 1
    binding=$(docker port "$container" 8080/tcp) || return 1
    [[ $binding == 127.0.0.1:* ]] || return 1
    site_url=http://$binding
    until request / >/dev/null 2>&1; do
        [[ $(docker inspect --format '{{.State.Running}}' "$container") == true ]] || return 1
        ((SECONDS < deadline)) || return 1
        sleep 0.2
    done
}

web_image_bases_are_pinned() {
    local bases
    bases=$(awk 'toupper($1) == "FROM" { print $2, tolower($3), $4 }' "$root/$dockerfile") || return 1
    [[ $bases == "$node as site"$'\n'"$caddy as runtime" ]]
}

web_image_context_is_allow_listed() {
    diff -u <(sed -E '/^[[:space:]]*(#|$)/d' "$root/$dockerfile.dockerignore") - <<'ALLOW_LIST'
*
!package.json
!pnpm-lock.yaml
!pnpm-workspace.yaml
!tsconfig.base.json
!src/frontend/**
!src/contracts/**
!src/mock-server/**
!design/mock/**
!design/fonts/**
!third_party/inter/**
!third_party/jetbrains-mono/**
!third_party/codemirror/**
!deploy/Caddyfile
**/node_modules
**/dist
**/.cert
ALLOW_LIST
}

web_image_leaves_out_state() {
    ((built)) || return 1
    local files statuses layer
    files=$(docker run --rm --name "$project-scan" --network none --entrypoint sh "$site_image" -ec '
        find / -xdev \( -name "$1" -o -path "*/.cert/*" -o -path "*/deploy/.env" \) -print
        test -s /repo/src/frontend/dist/index.html
    ' _ "$sentinel") || return 1
    [[ -z $files ]] || { printf '  A decoy file reached the site stage.\n'; return 1; }

    # Docker can store compressed layers. Scan both the saved metadata and each decoded layer.
    docker save "$web_image" -o "$work/image.tar" || return 1
    grep -aFf "$work/markers" "$work/image.tar" >/dev/null
    [[ $? == 1 ]] || return 1
    local layers
    layers=$(tar -xOf "$work/image.tar" manifest.json | jq -er '.[0].Layers[]') || return 1
    while IFS= read -r layer; do
        tar -xf "$work/image.tar" -C "$work" "$layer" || return 1
        tar -xOf "$work/$layer" | grep -aFf "$work/markers" >/dev/null
        statuses=("${PIPESTATUS[@]}")
        [[ ${statuses[*]} == '0 1' ]] || return 1
    done <<<"$layers"
}

web_image_serves_the_built_site() {
    ((built)) || return 1
    start_site "$project-site" && [[ $(request /) == 200 ]] &&
        grep -Fq 'id="app"' "$work/body" && grep -Eq '<script([[:space:]]|>)' "$work/body" || return 1
    local asset
    local -a assets=()
    mapfile -d '' -t assets < <(grep -Pizo \
        "[[:space:]](?:src|href)[[:space:]]*=[[:space:]]*\K(?:\"/[^\"\r\n]*\"|'/[^'\r\n]*'|/[^[:space:]<>]+)" "$work/body")
    ((${#assets[@]} > 0)) || return 1
    for asset in "${assets[@]}"; do
        case $asset in
            \"*\" | \'*\') asset=${asset:1:-1} ;;
        esac
        [[ $(request "$asset") == 200 ]] || { printf '  Asset did not return 200. %s\n' "$asset"; return 1; }
    done
}

web_image_fixes_the_server_address() {
    ((built)) && [[ -n $site_url ]] || return 1
    (cd "$root" && node - "$site_url") <<'NODE'
const assert = require('node:assert/strict');
const { chromium, expect } = require('@playwright/test');

async function checkServedSite() {
    const site = new URL(process.argv[2]);
    const getServer = new URL('/lgtm.v1.SessionService/GetServer', site).href;
    const browser = await chromium.launch();
    try {
        const page = await browser.newPage();
        let requestedOwnServer = false;
        page.on('request', request => {
            if (request.method() === 'POST' && request.url() === getServer) {
                requestedOwnServer = true;
            }
        });
        await page.goto(site.href, { timeout: 10_000 });
        const address = page.getByRole('textbox', { name: 'Server address' });
        const unavailable = page.getByRole('alert').filter({
            hasText: `No LGTM Studio server answers at ${site.host}.`,
        });
        await expect(unavailable.or(address)).toBeVisible({ timeout: 10_000 });
        assert.equal(await address.count(), 0, 'The served website asks for a server address.');
        assert.ok(requestedOwnServer, 'The served website never sent GetServer to its own origin.');
        await expect(unavailable).toBeVisible();
    } finally {
        await browser.close();
    }
}

checkServedSite().catch(error => {
    console.error(error.message);
    process.exitCode = 1;
});
NODE
}

web_image_carries_the_rules() {
    ((built)) || return 1
    [[ $(docker image inspect --format '{{.Config.User}}' "$web_image") == 10003:10003 ]] &&
        docker run --rm --name "$project-rules" --network none --entrypoint cat "$web_image" /etc/caddy/Caddyfile |
        cmp -s "$root/deploy/Caddyfile" -
}

web_image_runs_unprivileged() {
    ((built)) || return 1
    start_site "$project-hardening" --user 10003:10003 --read-only --cap-drop ALL \
        --security-opt no-new-privileges:true --tmpfs /tmp &&
        [[ $(request /) == 200 ]] && grep -Fq 'id="app"' "$work/body"
}

web_image_has_no_build_tools() {
    ((built)) || return 1
    docker run --rm --name "$project-tools" --network none --entrypoint sh "$web_image" -c \
        '! (command -v node || command -v pnpm || command -v npm)'
}

check web_image_bases_are_pinned
check web_image_context_is_allow_listed
if build_images; then
    built=1
else
    printf '  The site or runtime image could not be prepared.\n' >&2
    for log in "$work/site.log" "$work/build.log"; do
        [[ ! -f $log ]] || tail -n 18 "$log" >&2
    done
fi
check web_image_leaves_out_state
check web_image_serves_the_built_site
check web_image_fixes_the_server_address
check web_image_carries_the_rules
check web_image_runs_unprivileged
check web_image_has_no_build_tools
printf '%d passed, %d failed\n' "$passed" "$failed"
((failed == 0))
