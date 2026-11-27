#!/usr/bin/env bash
# Check the selected Compose model and discover Projects through the deployed website.
set +x
set -u -o pipefail
export LC_ALL=C

case_name=s4_projects_uses_existing_stack

fail() {
    printf 'not ok 1 - %s\n# %s\n' "$case_name" "$1"
    exit 1
}

compose_model() {
    # The command guard validates the selection. The fixture's model reader
    # resolves that same selection with both setup services included.
    sprint4_fixture_run bash -c '
        source "$s4_library" &&
            s4_guard_compose config --quiet &&
            s4_model
    '
}

assert_compose_model() {
    local model=$1
    jq -e '
        (.services | keys) == ["openbao", "openbao-init", "postgres", "prepare", "server", "web"]
        and (.services | to_entries | all(.[]; (.value.profiles // []) ==
            (if .key == "openbao-init" or .key == "prepare" then ["setup"] else [] end)))
    ' <<<"$model" >/dev/null 2>&1 ||
        fail 'Compose did not retain exactly the four running services and two setup services.'

    jq -e --arg tag "$LGTM_IMAGE_TAG" '
        .services.server.image == ("lgtm-server:" + $tag)
        and .services.web.image == ("lgtm-web:" + $tag)
    ' <<<"$model" >/dev/null 2>&1 ||
        fail 'Compose did not use the selected private tag for both images.'

    jq -e --arg port "$LGTM_WEB_PORT" '
        .services.web.ports == [{mode:"ingress", host_ip:"127.0.0.1", published:$port,
            target:8080, protocol:"tcp"}]
        and (.services | del(.web) | all(.[]; (.ports // []) == []))
    ' <<<"$model" >/dev/null 2>&1 ||
        fail 'Compose did not publish exactly one loopback binding at the fixture website port.'

    jq -e --arg project "$COMPOSE_PROJECT_NAME" '
        def storage: [.volumes[]? | select(.type == "volume") | [.source, .target]];
        def names: (.networks // {} | keys);
        .name == $project
        and (.volumes | keys) == ["pgdata"]
        and (.volumes.pgdata | .name == ($project + "_pgdata") and .external != true
            and (.driver // "local") == "local" and (.driver_opts // {}) == {})
        and (.services | to_entries | all(.[]; (.value | storage) ==
            (if .key == "postgres" or .key == "prepare" then
                [["pgdata", "/var/lib/postgresql/data"]] else [] end)))
        and .services.postgres.environment.PGDATA == "/var/lib/postgresql/data/pgdata"
        and (.networks | keys) == ["app", "data", "edge", "vault"]
        and (.networks | to_entries | all(.[];
            .value.name == ($project + "_" + .key) and .value.external != true
            and (.value.driver // "bridge") == "bridge"
            and (.value.internal // false) == (.key != "edge")))
        and (.services.postgres | names) == ["data"]
        and (.services.openbao | names) == ["data", "vault"]
        and (.services["openbao-init"] | names) == ["vault"]
        and (.services.prepare | names) == [] and .services.prepare.network_mode == "none"
        and (.services.server | names) == ["app", "data", "vault"]
        and (.services.web | names) == ["app", "edge"]
    ' <<<"$model" >/dev/null 2>&1 ||
        fail 'Compose did not retain the existing pgdata storage and private network topology.'

    jq -e --arg root "$S4_CANDIDATE_ROOT" '
        (.services.server.build | [.context, .dockerfile, .target]) ==
            [$root, "deploy/server.Dockerfile", "runtime"]
        and (.services.web.build | [.context, .dockerfile, .target]) ==
            [$root, "deploy/website.Dockerfile", "runtime"]
        and .services.postgres.image ==
            "postgres:16.16-alpine@sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d"
        and .services.prepare.image == .services.postgres.image
        and .services.openbao.image ==
            "openbao/openbao:2.7.3@sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9"
        and .services["openbao-init"].image == .services.openbao.image
    ' <<<"$model" >/dev/null 2>&1 ||
        fail 'Compose did not use the existing build inputs and pinned store images.'
}

record_image() {
    local tag=$1 id
    id=$(awk -v tag="$tag" '$1 == "image" && $2 == tag {print $3}' "$S4_FIXTURE_DIR/objects") &&
        [[ $id =~ ^sha256:[0-9a-f]{64}$ ]] || fail 'The fixture did not record the selected image identity.'
    printf '# %s %s\n' "$tag" "$id"
}

session_call() {
    local procedure=$1
    shift
    sprint4_fixture_run curl -q --silent --max-time 15 \
        --header 'Content-Type: application/json' --header 'Connect-Protocol-Version: 1' \
        --data-binary @- --write-out $'\n%{http_code}' "$@" \
        "http://127.0.0.1:$LGTM_WEB_PORT/lgtm.v1.SessionService/$procedure" 2>/dev/null
}

s4_projects_uses_existing_stack() {
    local model launcher address line jar response
    model=$(compose_model 2>/dev/null) || fail 'The guarded fixture could not resolve the Compose model.'
    assert_compose_model "$model"
    printf '# The private Compose model matches the existing stack.\n'

    if ! launcher=$(sprint4_fixture_run bash "$S4_CANDIDATE_ROOT/deploy/up.sh" 2>&1); then
        # Report only a known phase marker. The launcher can include container output.
        printf '%s\n' "$launcher" |
            awk '/^phase (database|seal|vault|images|server|web|ready) (start|done)$/ {last=$0}
                END {if (last != "") print "# " last}'
        fail 'The normal launcher could not start the selected private stack.'
    fi
    address="the website is at http://127.0.0.1:$LGTM_WEB_PORT/"
    for line in 'phase server done' 'phase web done' "$address"; do
        grep -Fxq -- "$line" <<<"$launcher" || fail 'The launcher did not report both services ready at the fixture address.'
    done
    printf '# %s\n' 'phase server done' 'phase web done' "$address"
    record_image "lgtm-server:$LGTM_IMAGE_TAG"
    record_image "lgtm-web:$LGTM_IMAGE_TAG"

    jar="$S4_FIXTURE_DIR/compose-session.cookies"
    response=$(printf '%s' "$LGTM_WEB_PASSWORD" | jq -Rsc '{password: .}' |
        session_call SignIn --cookie-jar "$jar") &&
        [[ ${response##*$'\n'} == 200 ]] &&
        jq -e '.result == "SIGN_IN_RESULT_SIGNED_IN" or .result == 1' \
            <<<"${response%$'\n'*}" >/dev/null 2>&1 ||
        fail 'The configured password did not sign in through the deployed website.'
    response=$(session_call GetServer --cookie "$jar" <<<'{}') &&
        [[ ${response##*$'\n'} == 200 ]] &&
        jq -e '.signedIn == true and .product == "lgtm-studio" and .sample != true and .contract == 1' \
            <<<"${response%$'\n'*}" >/dev/null 2>&1 ||
        fail 'GetServer did not report a signed-in real server at website contract revision 1.'
    jq -e '(.services // []) | index("lgtm.v1.ProjectService") != null' \
        <<<"${response%$'\n'*}" >/dev/null 2>&1 ||
        fail 'The signed-in GetServer response did not advertise lgtm.v1.ProjectService through the deployed website.'
}

root=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P) || exit 1
printf '1..1\n'
[[ $# -le 1 ]] || fail 'Supply at most one candidate commit.'
source "$root/deploy/test/sprint4-fixture.sh" ||
    fail 'The private fixture library could not be loaded.'
# Container users need to read the tracked files mounted from the private checkout.
umask 022
sprint4_fixture_open "$root" "${1:-HEAD}" && sprint4_select 4 ||
    fail 'A private candidate stack could not be selected.'
umask 077
s4_projects_uses_existing_stack
sprint4_fixture_close || fail 'The private candidate stack could not be cleaned up.'
printf 'ok 1 - %s\n' "$case_name"
