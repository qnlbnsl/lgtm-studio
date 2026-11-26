#!/usr/bin/env bash
# This suite builds both server stages from a tracked snapshot with decoy credentials.
set -u -o pipefail
export LC_ALL=C

source "$(dirname -- "${BASH_SOURCE[0]}")/web-testlib.sh"
golang=golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab
distroless=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
work=$(mktemp -d) || exit 1
project=lgtm-s2w-$(web_fixture_random 8)
build_image=$project:build
server_image=$project:runtime
snapshot=$work/checkout
dockerfile=deploy/server.Dockerfile
server_main=src/server/cmd/lgtm-server/main.go
export BUILDX_CONFIG=$work/buildx
web_passed=0 web_failed=0

cleanup() {
    docker rm -fv "$project-scan" "$project-shell" "$project-password" >/dev/null 2>&1 || true
    for image in "$build_image" "$server_image"; do
        if docker image inspect "$image" >/dev/null 2>&1; then
            docker image rm "$image" >/dev/null 2>&1 || true
        fi
    done
    rm -rf -- "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

server_image_bases_are_pinned() {
    local bases
    bases=$(awk 'toupper($1) == "FROM" { print $2, tolower($3), $4 }' "$web_root/$dockerfile") || return 1
    [[ $bases == "$golang as build"$'\n'"$distroless as runtime" ]]
}

server_image_context_is_allow_listed() {
    local actual
    actual=$(sed -E '/^[[:space:]]*(#|$)/d' "$web_root/$dockerfile.dockerignore") || return 1
    [[ $actual == $'*\n!go.mod\n!go.sum\n!src/server/**\n!src/contracts/gen/go/**\n**/*_test.go' ]]
}

build_images() {
    [[ -f $web_root/$dockerfile && -f $web_root/$dockerfile.dockerignore ]] || return 1
    [[ -f $web_root/$server_main ]] && git -C "$web_root" ls-files --error-unmatch "$server_main" >/dev/null 2>&1 ||
        { printf '  The image checks require the tracked server entry point from Server item 9.\n' >&2; return 1; }

    mkdir -p "$snapshot/deploy/.state" || return 1
    (
        cd "$web_root" || exit 1
        git ls-files -z -- . ':(exclude).env' ':(exclude)deploy/.env' ':(exclude)deploy/.state/**' |
            tar -c --ignore-failed-read --no-recursion --null -T - |
            tar -xf - -C "$snapshot"
    ) || return 1
    # The Dockerfile and its ignore file can be new when the builder runs this suite.
    cp -- "$web_root/$dockerfile" "$web_root/$dockerfile.dockerignore" "$snapshot/deploy/" || return 1
    local path marker
    for path in deploy/.env "deploy/.state/sentinel-$(web_fixture_random 8)"; do
        marker=$(web_fixture_random 16) || return 1
        [[ $marker =~ ^[0-9a-f]{32}$ ]] || return 1
        printf '%s\n' "$marker" >>"$work/markers"
        printf '%s\n' "$marker" >"$snapshot/$path"
    done
    docker build --target build --tag "$build_image" --file "$snapshot/$dockerfile" "$snapshot" >"$work/build.log" 2>&1 &&
        docker build --tag "$server_image" --file "$snapshot/$dockerfile" "$snapshot" >"$work/runtime.log" 2>&1
}

server_image_leaves_out_state() {
    ((built)) || return 1
    local files layers layer
    local -a statuses
    files=$(docker run --rm --name "$project-scan" --network none -i --entrypoint sh "$build_image" -ec '
        find / -xdev -name "sentinel-*" -print
        test -f "$PWD/go.mod"
        test -x /out/lgtm-server
        status=0
        grep -r -F -f /dev/stdin "$PWD" >/dev/null || status=$?
        test "$status" = 1
    ' <"$work/markers") || return 1
    [[ -z $files ]] || return 1
    docker save -o "$work/image.tar" "$server_image" || return 1
    grep -aFf "$work/markers" "$work/image.tar" >/dev/null
    [[ $? == 1 ]] || return 1

    # The saved image can contain compressed layers.
    layers=$(tar -xOf "$work/image.tar" manifest.json | jq -er '.[0].Layers[]') || return 1
    while IFS= read -r layer; do
        tar -xf "$work/image.tar" -C "$work" "$layer" || return 1
        tar -xOf "$work/$layer" | grep -aFf "$work/markers" >/dev/null
        statuses=("${PIPESTATUS[@]}")
        [[ ${statuses[*]} == '0 1' ]] || return 1
    done <<<"$layers"
}

server_image_adds_only_the_server_program() {
    ((built)) || return 1
    local base_layers runtime_layers added_layers layer entries
    local paths='' archive=$work/runtime-layers.tar
    if ! docker image inspect "$distroless" >/dev/null 2>&1; then
        docker pull "$distroless" >/dev/null || return 1
    fi
    base_layers=$(docker image inspect --format '{{json .RootFS.Layers}}' "$distroless") || return 1
    runtime_layers=$(docker image inspect --format '{{json .RootFS.Layers}}' "$server_image") || return 1
    if ! jq -e --argjson base "$base_layers" '
        ($base | length) > 0 and .[0:($base | length)] == $base
    ' <<<"$runtime_layers" >/dev/null; then
        printf '  The runtime image does not start with the pinned base layers.\n' >&2
        return 1
    fi

    docker save -o "$archive" "$server_image" || return 1
    added_layers=$(tar -xOf "$archive" manifest.json |
        jq -r --argjson base "$base_layers" '.[0].Layers[($base | length):][]') || return 1
    while IFS= read -r layer; do
        [[ -n $layer ]] || continue
        tar -xOf "$archive" -- "$layer" >"$work/runtime-layer.tar" || return 1
        # A copy may create parent directories. Every other entry counts, including deletions.
        entries=$(tar -tvf "$work/runtime-layer.tar" --numeric-owner --quoting-style=escape |
            awk '
                $1 ~ /^d/ { next }
                $1 ~ /^-/ && NF == 6 { sub(/^\.\//, "", $6); print $6; next }
                { print "<unexpected archive entry>" }
            ') || return 1
        [[ -z $entries ]] || paths+="$entries"$'\n'
    done <<<"$added_layers"
    if [[ $paths != $'usr/local/bin/lgtm-server\n' ]]; then
        printf '  The server program must be the only file added to the pinned base.\n' >&2
        return 1
    fi
}

server_image_runs_unprivileged() {
    ((built)) && docker image inspect "$server_image" |
        jq -e '.[0].Config |
            .User == "10001:10001" and .Entrypoint == ["/usr/local/bin/lgtm-server"]
            and ((.Cmd // []) == [])' >/dev/null
}

server_image_has_no_shell() {
    ((built)) || return 1
    local output status=0
    output=$(docker run --rm --name "$project-shell" --network none --entrypoint /bin/sh "$server_image" -c true 2>&1) ||
        status=$?
    [[ $status != 0 && $output == *'/bin/sh'* && $output == *'no such file or directory'* ]]
}

server_image_refuses_without_password() {
    ((built)) || return 1
    local output status=0
    output=$(timeout 10 docker run --rm --name "$project-password" --network none \
        --read-only --cap-drop ALL "$server_image" 2>&1) || status=$?
    ((status != 0 && status < 124)) && [[ $output == *LGTM_WEB_PASSWORD* ]]
}

web_check server_image_bases_are_pinned
web_check server_image_context_is_allow_listed
built=0
if build_images; then
    built=1
else
    for log in "$work/build.log" "$work/runtime.log"; do
        [[ ! -f $log ]] || tail -n 12 "$log" >&2
    done
fi
web_check server_image_leaves_out_state
web_check server_image_adds_only_the_server_program
web_check server_image_runs_unprivileged
web_check server_image_has_no_shell
web_check server_image_refuses_without_password
web_finish
