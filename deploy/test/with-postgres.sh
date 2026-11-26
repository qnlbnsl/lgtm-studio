#!/usr/bin/env bash
set +x
set -euo pipefail
umask 077

if [[ $# -lt 2 || $1 != -- ]]; then
    printf 'Usage: %s -- command [argument ...]\n' "${0##*/}" >&2
    exit 2
fi
shift

image=postgres:16.16-alpine@sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d
database=lgtm_store_test
login=lgtm_test
scratch=$(mktemp -d)
name="lgtm-s1v-test-${scratch##*.}"
label="lgtm.s1v.fixture=$name"
child_pid=
volume=
container_started=

cleanup() {
    local status=$? containers failed=0
    local -a ids
    trap - EXIT
    trap '' INT TERM

    if [[ -n $child_pid ]]; then
        kill -TERM -- "-$child_pid" 2>/dev/null || kill -TERM "$child_pid" 2>/dev/null || true
    fi
    if [[ -n $container_started ]]; then
        if containers=$(docker ps -aq --filter "label=$label" 2>/dev/null); then
            if [[ -n $containers ]]; then
                mapfile -t ids <<< "$containers"
                docker rm -f "${ids[@]}" >/dev/null 2>&1 || failed=1
            fi
        else
            failed=1
        fi
    fi
    if [[ -n $volume ]]; then
        docker volume rm "$volume" >/dev/null 2>&1 || failed=1
    fi
    rm -rf -- "$scratch" || failed=1

    if (( failed != 0 )); then
        printf 'Could not remove all disposable PostgreSQL resources.\n' >&2
        (( status != 0 )) || status=1
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

fail() {
    printf '%s\n' "$*" >&2
    exit 1
}

random_hex() {
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
    printf '\n'
}

mkdir -- "$scratch/credentials"
random_hex > "$scratch/credentials/admin.password"
random_hex > "$scratch/credentials/test.password"
password=$(< "$scratch/credentials/test.password")

volume=$(docker volume create --label "$label" "$name" 2>"$scratch/docker.err") ||
    fail 'Could not create the disposable PostgreSQL volume.'
container_started=yes
docker run --detach --name "$name" --label "$label" \
    --publish 127.0.0.1::5432 \
    --env POSTGRES_PASSWORD_FILE=/run/lgtm-test/admin.password \
    --env POSTGRES_INITDB_ARGS='--auth-host=scram-sha-256 --auth-local=scram-sha-256' \
    --env POSTGRES_HOST_AUTH_METHOD=scram-sha-256 \
    --mount "type=bind,source=$scratch/credentials,target=/run/lgtm-test,readonly" \
    --mount "type=volume,source=$volume,target=/var/lib/postgresql/data" "$image" \
    > /dev/null 2>"$scratch/docker.err" ||
    fail 'Could not start disposable PostgreSQL.'

admin_sql() {
    timeout 5 docker exec -i "$name" sh -ec '
        export PGPASSWORD="$(cat /run/lgtm-test/admin.password)"
        export TEST_PASSWORD="$(cat /run/lgtm-test/test.password)"
        exec psql -X -h 127.0.0.1 -U postgres -d postgres -v ON_ERROR_STOP=1 "$@"
    ' sh "$@" 2>"$scratch/psql.err"
}

deadline=$((SECONDS + 60))
until [[ $(admin_sql -Atqc 'SELECT 1' </dev/null) == 1 ]]; do
    (( SECONDS < deadline )) || fail 'Disposable PostgreSQL did not become ready.'
    sleep 1
done

admin_sql -v "login=$login" -v "database=$database" > /dev/null <<'SQL' ||
\getenv test_password TEST_PASSWORD
CREATE ROLE :"login" LOGIN CREATEDB NOSUPERUSER NOCREATEROLE PASSWORD :'test_password';
CREATE DATABASE :"database" OWNER :"login";
SQL
    fail 'Could not create the disposable PostgreSQL test login and database.'

binding=$(docker port "$name" 5432/tcp 2>"$scratch/docker.err") ||
    fail 'Could not find the disposable PostgreSQL port.'
port=${binding##*:}
[[ $port =~ ^[0-9]+$ && $port -gt 0 ]] ||
    fail 'Could not find the disposable PostgreSQL port.'

# A separate session lets the wrapper stop the command and its descendants on interruption.
LGTM_TEST_POSTGRES="postgresql://$login:$password@127.0.0.1:$port/$database?sslmode=disable" \
    setsid -- "$@" <&0 &
child_pid=$!
child_status=0
wait "$child_pid" || child_status=$?
child_pid=
exit "$child_status"
