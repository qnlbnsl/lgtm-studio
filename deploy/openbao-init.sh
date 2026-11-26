#!/bin/sh
set +x
set -eu
umask 077
export LC_ALL=C
unset BAO_TOKEN

response=/admin/init-response.json
credential=/server/openbao.json
policy=/openbao-server-policy.hcl
role=auth/approle/role/lgtm-server
incomplete_message='vault is initialized but bootstrap state is incomplete'
active_message='OpenBao did not become active'
repair_message='server vault credential needs repair'

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

exists() {
    [ -e "$1" ] || [ -L "$1" ]
}

protected_file() {
    [ ! -L "$1" ] && [ -f "$1" ] &&
        [ "$(stat -c '%u:%g %a' "$1")" = "$2 400" ]
}

elapsed() {
    read -r uptime rest < /proc/uptime
    printf '%s\n' "${uptime%%.*}"
}

# Use the initialization endpoint so an uninitialized or sealed daemon can answer.
get_initialization_state() {
    deadline=$(($(elapsed) + 60))
    while [ "$(elapsed)" -lt "$deadline" ]; do
        status=0
        timeout 3 bao operator init -status >/dev/null 2>&1 || status=$?
        case "$status" in
            0|2) return 0 ;;
        esac
        sleep 1
    done
    fail "$active_message"
}

wait_active() {
    deadline=$(($(elapsed) + 60))
    while [ "$(elapsed)" -lt "$deadline" ]; do
        if timeout 3 bao status -format=json >/dev/null 2>&1 &&
            [ "$(elapsed)" -lt "$deadline" ]; then
            return
        fi
        sleep 1
    done
    fail "$active_message"
}

# Validate the entire pinned formatter output for one recovery share before
# extracting the token. A valid token line alone proves nothing about the file.
read_root_token() {
    protected_file "$response" 10002:10002 || return 1
    awk '
        NR == 1 && /^\{$/ { valid++ }
        NR == 2 && /^  "unseal_keys_b64": \[\],$/ { valid++ }
        NR == 3 && /^  "unseal_keys_hex": \[\],$/ { valid++ }
        NR == 4 && /^  "unseal_shares": 1,$/ { valid++ }
        NR == 5 && /^  "unseal_threshold": 1,$/ { valid++ }
        NR == 6 && /^  "recovery_keys_b64": \[$/ { valid++ }
        NR == 7 && /^    "[A-Za-z0-9+\/]+="$/ && length($1) == 46 { valid++ }
        NR == 8 && /^  \],$/ { valid++ }
        NR == 9 && /^  "recovery_keys_hex": \[$/ { valid++ }
        NR == 10 && /^    "[0-9a-f]+"$/ && length($1) == 66 { valid++ }
        NR == 11 && /^  \],$/ { valid++ }
        NR == 12 && /^  "recovery_keys_shares": 1,$/ { valid++ }
        NR == 13 && /^  "recovery_keys_threshold": 1,$/ { valid++ }
        NR == 14 && /^  "root_token": "[A-Za-z0-9._-]+"$/ { valid++ }
        NR == 15 && /^\}$/ { valid++ }
        END { exit !(NR == 15 && valid == 15) }
    ' "$response" || return 1
    root_token=$(sed -n 's/^  "root_token": "\([A-Za-z0-9._-][A-Za-z0-9._-]*\)"$/\1/p' "$response")
    [ -n "$root_token" ]
}

as_root() {
    BAO_TOKEN="$root_token" bao "$@"
}

quiet_root() {
    as_root "$@" 2>/dev/null
}

mount_entry() {
    awk -v path="$1/" '
        $0 == "  \"" path "\": {" { entry = 1 }
        entry { print }
        entry && /^  },?$/ { exit }
    '
}

ensure_mounts() {
    mounts=$(quiet_root secrets list -format=json) || return 1
    kv=$(mount_entry lgtm <<EOF_MOUNTS
$mounts
EOF_MOUNTS
    )
    if [ -n "$kv" ]; then
        printf '%s\n' "$kv" | grep -Eq '^    "type": "kv",?$' &&
            printf '%s\n' "$kv" | grep -Eq '^      "version": "2",?$' || return 1
    else
        quiet_root secrets enable -path=lgtm -version=2 kv >/dev/null || return 1
    fi

    mounts=$(quiet_root auth list -format=json) || return 1
    approle=$(mount_entry approle <<EOF_MOUNTS
$mounts
EOF_MOUNTS
    )
    if [ -n "$approle" ]; then
        printf '%s\n' "$approle" | grep -Eq '^    "type": "approle",?$' || return 1
    else
        quiet_root auth enable approle >/dev/null || return 1
    fi
}

identifier() {
    case "$1" in
        ''|*[!A-Za-z0-9._-]*) return 1 ;;
    esac
}

validate_login() {
    protected_file "$1" 10001:10001 &&
        [ "$(wc -l < "$1")" -eq 1 ] &&
        grep -Eq '^\{[[:blank:]]*("role_id"[[:blank:]]*:[[:blank:]]*"[A-Za-z0-9._-]+"[[:blank:]]*,[[:blank:]]*"secret_id"|"secret_id"[[:blank:]]*:[[:blank:]]*"[A-Za-z0-9._-]+"[[:blank:]]*,[[:blank:]]*"role_id")[[:blank:]]*:[[:blank:]]*"[A-Za-z0-9._-]+"[[:blank:]]*\}$' "$1" &&
        bao write auth/approle/login - < "$1" >/dev/null 2>&1
}

protect() {
    chmod 0400 "$1"
    chown "$2" "$1"
    sync "$1"
}

publish() {
    mv -T -- "$1" "$2"
    sync "${2%/*}"
}

response_temp=
credential_temp=
list_dir=
cleanup() {
    [ -z "$response_temp" ] || rm -f -- "$response_temp"
    [ -z "$credential_temp" ] || rm -f -- "$credential_temp"
    [ -z "$list_dir" ] || rm -rf -- "$list_dir"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

get_initialization_state
if [ "$status" = 2 ]; then
    if exists "$response" || exists "$credential"; then
        fail 'state directory is ahead of the database'
    fi
    response_temp=$(mktemp /admin/.init-response.XXXXXXXX)
    timeout 60 bao operator init -format=json -recovery-shares=1 -recovery-threshold=1 \
        > "$response_temp" 2>/dev/null || fail "$incomplete_message"
    protect "$response_temp" 10002:10002
    publish "$response_temp" "$response"
    response_temp=
fi

read_root_token || fail "$incomplete_message"
wait_active
if exists "$credential"; then
    validate_login "$credential" || fail "$repair_message"
    exit 0
fi

sync "$response" /admin
ensure_mounts || fail "$incomplete_message"
quiet_root policy write lgtm-server "$policy" >/dev/null || fail "$incomplete_message"
quiet_root write "$role" secret_id_ttl=0 secret_id_num_uses=0 token_policies=lgtm-server \
    token_type=batch token_ttl=15m token_max_ttl=15m token_no_default_policy=true \
    >/dev/null || fail "$incomplete_message"
quiet_root read "$role" >/dev/null || fail "$incomplete_message"

list_dir=$(mktemp -d)
result=0
as_root list -format=json "$role/secret-id" > "$list_dir/out" 2> "$list_dir/err" || result=$?
[ "$result" -eq 2 ] && [ ! -s "$list_dir/err" ] &&
    printf '{}\n' | cmp -s - "$list_dir/out" || fail "$incomplete_message"

role_id=$(quiet_root read -field=role_id "$role/role-id") || fail "$incomplete_message"
identifier "$role_id" || fail "$incomplete_message"
secret_id=$(quiet_root write -f -field=secret_id "$role/secret-id") || fail "$incomplete_message"
identifier "$secret_id" || fail "$incomplete_message"
credential_temp=$(mktemp /server/.openbao.XXXXXXXX)
printf '{"role_id":"%s","secret_id":"%s"}\n' "$role_id" "$secret_id" > "$credential_temp"
protect "$credential_temp" 10001:10001
validate_login "$credential_temp" || fail "$repair_message"
publish "$credential_temp" "$credential"
credential_temp=
