#!/bin/sh
set +x
set -eu
umask 077

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

private_directory() {
    [ ! -L "$1" ] || fail 'Invalid stack state directory.'
    mkdir -p -- "$1"
    chmod 0700 -- "$1"
}

file_exists() {
    [ -e "$1" ] || [ -L "$1" ]
}

password_diagnostic='restore the matching PostgreSQL password files'

valid_password() {
    [ ! -L "$1" ] && [ -f "$1" ] &&
        [ "$(stat -c '%u:%g %a %s' "$1")" = "$2 ${3#0} 65" ] &&
        LC_ALL=C grep -Eq '^[0-9a-f]{64}$' "$1" &&
        [ "$(wc -l < "$1")" -eq 1 ]
}

prepare_password() {
    file=$1
    owner=$2
    mode=$3
    if file_exists "$file"; then
        valid_password "$file" "$owner" "$mode" || fail "$password_diagnostic"
        return
    fi
    [ "$fresh" = yes ] || fail "$password_diagnostic"
    private_temp "$file"
    od -An -N32 -tx1 /dev/urandom > "$temp"
    password=$(tr -d ' \n' < "$temp")
    [ "${#password}" -eq 64 ] || fail 'Could not generate the PostgreSQL password.'
    printf '%s\n' "$password" > "$temp"
    publish_file "$file" "$owner" "$mode"
}

private_temp() {
    temp=$(mktemp "${1}.XXXXXXXX")
    trap 'rm -f -- "$temp"' EXIT
    trap 'exit 1' HUP INT TERM
}

publish_file() {
    chown "$2" "$temp"
    chmod "$3" "$temp"
    sync "$temp"
    mv -T -- "$temp" "$1"
    sync "${1%/*}"
    trap - EXIT HUP INT TERM
}

prepare_db_files() {
    data=/var/lib/postgresql/data
    chown 70:70 "$data"
    chmod 0700 "$data"
    fresh=yes
    if [ -n "$(ls -A "$data")" ]; then
        fresh=no
    fi
    private_directory /state
    for folder in bao admin server; do
        private_directory "/state/$folder"
        chown 0:0 "/state/$folder"
    done
    prepare_password /state/admin/postgres-admin.password 70:70 0400
    prepare_password /state/server/postgres-app.password 70:10001 0440
    prepare_password /state/bao/postgres-bao.password 70:10002 0440
}

prepare_seal_file() {
    key=/state/bao/unseal.key
    if file_exists "$key"; then
        [ ! -L "$key" ] && [ -f "$key" ] &&
            [ "$(stat -c '%u:%g %a %s' "$key")" = "10002:10002 400 32" ] ||
            fail 'seal key invalid. Restore the matching 32-byte seal key.'
        return
    fi
    if [ "$1" = t ] || file_exists /state/admin/init-response.json || file_exists /state/server/openbao.json; then
        fail 'seal key missing for existing vault'
    fi
    private_temp "$key"
    head -c 32 /dev/urandom > "$temp"
    [ "$(wc -c < "$temp")" -eq 32 ] || fail 'Could not generate the seal key.'
    publish_file "$key" 10002:10002 0400
}

compose() {
    docker compose -f "$here/compose.yaml" "$@"
}

offline_prepare() {
    compose --profile setup run --rm --no-deps -T prepare "$@"
}

pg_exec() {
    compose exec -T --user 70:70 postgres /bin/sh -ec "$@"
}

reconcile_db() {
    pg_exec '
        n=0
        until pg_isready -q -t 1 -h 127.0.0.1 -U postgres -d postgres; do
            n=$((n+1))
            [ "$n" -lt 30 ] || exit 1
            sleep 1
        done
        PGPASSWORD=$(cat /run/lgtm-db/postgres-admin.password)
        LGTM_APP_PASSWORD=$(cat /run/lgtm-db/postgres-app.password)
        LGTM_BAO_PASSWORD=$(cat /run/lgtm-db/postgres-bao.password)
        export PGPASSWORD LGTM_APP_PASSWORD LGTM_BAO_PASSWORD PGCONNECT_TIMEOUT=5
        psql -Xwq -h 127.0.0.1 -U postgres -d postgres -v ON_ERROR_STOP=1
        for service in app bao; do
            password=$(cat "/run/lgtm-db/postgres-$service.password")
            PGPASSWORD=$password psql -XwqAt -h 127.0.0.1 -U "lgtm_$service" -d "lgtm_$service" \
                -v ON_ERROR_STOP=1 -c "SELECT 1"
        done
    ' < "$here/postgres-init.sql" >/dev/null 2>&1 ||
        fail 'PostgreSQL reconciliation failed. Check the pg_hba SCRAM rules or restore the matching PostgreSQL password files.'
}

seal() {
    rows=$(pg_exec '
        PGPASSWORD=$(cat /run/lgtm-db/postgres-bao.password)
        export PGPASSWORD PGCONNECT_TIMEOUT=5
        exec psql -XwqAt -h 127.0.0.1 -U lgtm_bao -d lgtm_bao -v ON_ERROR_STOP=1
    ' 2>/dev/null <<'SQL'
SELECT to_regclass('public.openbao_kv_store') IS NOT NULL AS has_table \gset
\if :has_table
SELECT EXISTS (SELECT FROM public.openbao_kv_store);
\else
SELECT false;
\endif
SQL
    ) || fail 'Could not check OpenBao storage in PostgreSQL.'
    case "$rows" in
        t|f) offline_prepare _seal "$rows" ;;
        *) fail 'Could not check OpenBao storage in PostgreSQL.' ;;
    esac
}

case "${1:-}" in
    _prepare-db)
        prepare_db_files
        exit
        ;;
    _seal)
        prepare_seal_file "$2"
        exit
        ;;
    prepare-db|reconcile-db|seal) ;;
    *) fail 'Usage: stack-prepare.sh prepare-db|reconcile-db|seal.' ;;
esac

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
state=${LGTM_STACK_STATE_DIR:-"$here/.state"}
case "$state" in
    /*) ;;
    *) fail 'LGTM_STACK_STATE_DIR must be absolute.' ;;
esac
if [ "$1" = prepare-db ]; then
    mkdir -p -m 0700 -- "$state"
fi
LGTM_STACK_STATE_DIR=$(CDPATH='' cd -- "$state" && pwd -P)
export LGTM_STACK_STATE_DIR

case "$1" in
    prepare-db) offline_prepare _prepare-db ;;
    reconcile-db) reconcile_db ;;
    seal) seal ;;
esac
