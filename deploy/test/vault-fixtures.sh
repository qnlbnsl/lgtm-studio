#!/usr/bin/env bash
# Frozen assertions of work item 8, "Provide the real test database and rollout hook".
#
# Run from anywhere: bash deploy/test/vault-fixtures.sh
# The first assertion starts disposable PostgreSQL instances through
# deploy/test/with-postgres.sh. The second one tests deploy/test/mutate-feed-lock.sh
# in a scratch copy of the test folder, with disposable stand-ins for the
# Server script. No password or connection string is printed.
set -u
set -o pipefail
export LC_ALL=C

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
wrapper=$root/deploy/test/with-postgres.sh
hook=$root/deploy/test/mutate-feed-lock.sh
pg_digest=sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d

passed=0
failed=0
expect() {
    local description=$1
    shift
    if "$@" >/dev/null 2>&1; then
        printf 'PASS %s\n' "$description"
        passed=$((passed + 1))
    else
        printf 'FAIL %s\n' "$description"
        failed=$((failed + 1))
    fi
}

work=$(mktemp -d)
pids=()
cleanup() {
    local pid
    for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null; done
    rm -rf -- "$work"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# test_database_is_disposable
# ---------------------------------------------------------------------------

# The child command. It reads the setting from its own environment, uses it
# without printing it, and writes only facts to the file named by $1.
cat >"$work/child.sh" <<'CHILD'
#!/usr/bin/env bash
set -u
facts=$1
mode=${2:-facts}
url=${LGTM_TEST_POSTGRES:-}
[[ -n $url ]] || { echo "unset" >"$facts"; exit 3; }
re='^postgres(ql)?://([^:@/]+):([^@/]*)@([^:/?]+|\[[^]]+\]):([0-9]+)/([^?]+)'
[[ $url =~ $re ]] || { echo "unparsable" >"$facts"; exit 3; }
user=${BASH_REMATCH[2]} pass=${BASH_REMATCH[3]} host=${BASH_REMATCH[4]}
port=${BASH_REMATCH[5]} db=${BASH_REMATCH[6]}
{
    echo "host=$host"
    echo "port=$port"
    echo "database_in_setting=$db"
    echo "password_length=${#pass}"
} >"$facts"
printf '%s' "$pass" >"$facts.password"
q() { PGPASSWORD=$pass psql --no-psqlrc -qtAX -h "${host//[\[\]]/}" -p "$port" -U "$user" -d "$1" -v ON_ERROR_STOP=1 -c "$2"; }
case $mode in
facts)
    {
        echo "current_database=$(q "$db" 'select current_database()')"
        echo "owner_is_login=$(q "$db" "select pg_get_userbyid(datdba) = current_user from pg_database where datname = current_database()")"
        echo "createdb=$(q "$db" 'select rolcreatedb from pg_roles where rolname = current_user')"
        echo "scratch_created=$(q "$db" 'create database lgtm_scratch_probe' >/dev/null 2>&1 && echo t || echo f)"
        q "$db" 'drop database if exists lgtm_scratch_probe' >/dev/null 2>&1
        echo "done=yes"
    } >>"$facts"
    : >"$facts.ready"
    for _ in $(seq 1 300); do [[ -e $facts.release ]] && break; sleep 0.2; done
    ;;
hold)
    : >"$facts.ready"
    sleep 120
    ;;
fail)
    : >"$facts.ready"
    exit 7
    ;;
esac
CHILD
chmod +x "$work/child.sh"

fact() { sed -n "s/^$1=//p" "$work/$2" 2>/dev/null | head -n 1; }

wait_for() { # <file> <pid>: waits until the file exists, the process ends or 60 seconds pass
    local file=$1 pid=$2 tries=300
    while [[ ! -e $file && $tries -gt 0 ]] && kill -0 "$pid" 2>/dev/null; do
        sleep 0.2
        tries=$((tries - 1))
    done
    [[ -e $file ]]
}

container_for_port() {
    docker ps --format '{{.ID}} {{.Ports}}' | grep -E "(^| |:)$1->5432/tcp" | head -n 1 | cut -d' ' -f1
}

# Run 1: the child reads the database and the wrapper is observed while it runs.
mkdir "$work/tmp1"
(
    TMPDIR=$work/tmp1 "$wrapper" -- "$work/child.sh" "$work/facts1" facts >"$work/out1" 2>&1
    echo $? >"$work/status1"
) &
pids+=($!)
wait_for "$work/facts1.ready" "${pids[-1]}"
sleep 0.5
port=$(fact port facts1)
password=$(cat "$work/facts1.password" 2>/dev/null)
cid=""
[[ -n $port ]] && cid=$(container_for_port "$port")
if [[ -n $cid ]]; then
    docker inspect "$cid" >"$work/inspect1" 2>/dev/null
    docker logs "$cid" >"$work/logs1" 2>&1
    volumes=$(docker inspect --format '{{range .Mounts}}{{if eq .Type "volume"}}{{.Name}} {{end}}{{end}}' "$cid")
    ports=$(docker inspect --format '{{json .NetworkSettings.Ports}}' "$cid")
    image=$(docker inspect --format '{{.Config.Image}} {{.Image}}' "$cid")
fi
ps -eo args >"$work/ps1" 2>/dev/null
ss -ltnH 2>/dev/null >"$work/ss1"
: >"$work/facts1.release"
wait "${pids[-1]}" 2>/dev/null
status1=$(cat "$work/status1" 2>/dev/null || echo missing)

expect "the wrapper passes the setting to the child" test -s "$work/facts1" -a "$(fact done facts1)" = yes
expect "the child connects to the maintenance database lgtm_store_test" \
    test "$(fact current_database facts1)" = lgtm_store_test -a "$(fact database_in_setting facts1)" = lgtm_store_test
expect "the test login owns the database" test "$(fact owner_is_login facts1)" = t
expect "the test login may create databases and can create a scratch database" \
    test "$(fact createdb facts1)" = t -a "$(fact scratch_created facts1)" = t
expect "the setting names a loopback host and a dynamic nonzero port" bash -c '
    [[ $1 =~ ^(127\.0\.0\.1|localhost|\[?::1\]?)$ && $2 =~ ^[0-9]+$ && $2 -gt 1024 && $2 -ne 5432 ]]
' _ "$(fact host facts1)" "$port"
expect "the password is a nonempty random value of at least 16 characters" test "${#password}" -ge 16
expect "the instance runs in a container started from the pinned image" bash -c '
    [[ -n $1 ]] && grep -Fq "$2" <<<"$1"
' _ "${image:-}" "$pg_digest"
expect "the container publishes only on loopback" bash -c '
    [[ -n $1 ]] && jq -e "
        to_entries as \$e | (\$e | length) > 0
        and ([\$e[].value[]?] | length) > 0
        and all(\$e[].value[]?; .HostIp == \"127.0.0.1\" or .HostIp == \"::1\")
    " <<<"$1" >/dev/null
' _ "${ports:-}"
expect "the host listener for the port is bound to loopback only" bash -c '
    lines=$(awk -v p=":$2\$" "\$4 ~ p {print \$4}" "$1")
    [[ -n $lines ]] && ! grep -Eq "^(0\.0\.0\.0|\*|\[::\]|::):" <<<"$lines"
' _ "$work/ss1" "$port"
expect "the password is absent from every process argument list" bash -c '
    [[ -n $2 ]] && ! grep -Fq -- "$2" "$1"
' _ "$work/ps1" "$password"
expect "the password is absent from the container configuration and logs" bash -c '
    [[ -n $3 && -s $1 ]] && ! grep -Fq -- "$3" "$1" "$2"
' _ "$work/inspect1" "$work/logs1" "$password"
expect "the password is absent from the wrapper output" bash -c '
    [[ -n $2 ]] && ! grep -Fq -- "$2" "$1" && ! grep -Eq "postgres(ql)?://[^ ]*:[^ ]*@" "$1"
' _ "$work/out1" "$password"
expect "the password reaches the container through a mounted file" bash -c '
    jq -e "
        (.[0].Config.Env | map(select(startswith(\"POSTGRES_PASSWORD=\"))) | length) == 0
        and ((.[0].Mounts | length) > 0)
    " "$1" >/dev/null
' _ "$work/inspect1"
expect "the child status is zero and the wrapper returns it" test "$status1" = 0
expect "no private file remains after a normal exit" test -z "$(ls -A "$work/tmp1")"
expect "the container and its volumes are gone after a normal exit" bash -c '
    [[ -n $1 ]] || exit 1
    ! docker ps -a --format "{{.ID}}" | grep -q "^${1:0:12}" &&
    for v in $2; do ! docker volume inspect "$v" >/dev/null 2>&1 || exit 1; done
' _ "$cid" "${volumes:-}"
expect "the port no longer accepts connections after a normal exit" bash -c '
    [[ -n $1 ]] && ! (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
' _ "$port"

# Run 2: the child fails with status 7.
mkdir "$work/tmp2"
TMPDIR=$work/tmp2 "$wrapper" -- "$work/child.sh" "$work/facts2" fail >"$work/out2" 2>&1
status2=$?
port2=$(fact port facts2)
expect "the wrapper returns the failing status of the child" test "$status2" -eq 7
expect "no private file remains after a failing child" test -z "$(ls -A "$work/tmp2")"
expect "the instance is gone after a failing child" bash -c '
    [[ -n $1 ]] && ! (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null &&
    [[ -z $(docker ps -aq --filter "publish=$1") ]]
' _ "$port2"

# Run 3 and 4: handled interruptions. Perl resets the ignored interrupt
# signal that a background job of a script starts with.
for sig in INT TERM; do
    n=$([[ $sig == INT ]] && echo 3 || echo 4)
    mkdir "$work/tmp$n"
    TMPDIR=$work/tmp$n perl -e '
        use POSIX qw(setsid); setsid();
        $SIG{INT} = "DEFAULT"; $SIG{TERM} = "DEFAULT";
        exec @ARGV' "$wrapper" -- "$work/child.sh" "$work/facts$n" hold >"$work/out$n" 2>&1 &
    wpid=$!
    pids+=($wpid)
    wait_for "$work/facts$n.ready" "$wpid"
    portn=$(fact port "facts$n")
    cidn=$(container_for_port "$portn")
    kill "-$sig" "$wpid" 2>/dev/null
    for _ in $(seq 1 150); do kill -0 "$wpid" 2>/dev/null || break; sleep 0.2; done
    exited=yes
    kill -0 "$wpid" 2>/dev/null && exited=no
    pkill -f -- "$work/child.sh $work/facts$n" 2>/dev/null
    expect "after $sig the wrapper exits by itself" test "$exited" = yes -a -n "$cidn"
    expect "after $sig no private file remains" test -z "$(ls -A "$work/tmp$n")"
    expect "after $sig the container is removed" bash -c '
        [[ -n $1 && -n $2 ]] && [[ -z $(docker ps -aq --filter "id=$1") ]] &&
        ! (exec 3<>"/dev/tcp/127.0.0.1/$2") 2>/dev/null
    ' _ "$cidn" "$portn"
done

# ---------------------------------------------------------------------------
# mutation_hook_passes_through_server_status
# ---------------------------------------------------------------------------

# A scratch tree that looks like the repository to the hook: the test folder
# is copied, and the Server script is a disposable stand-in.
tree=$work/tree
mkdir -p "$tree/deploy" "$tree/src/server/internal/store"
cp -R "$root/deploy/test" "$tree/deploy/test"
( cd "$tree" && git init -q . ) >/dev/null 2>&1
server_script=$tree/src/server/internal/store/check-feed-lock.sh
sentinel="postgres://sentinel_user:sentinel-secret-9d41@127.0.0.1:5999/lgtm_store_test"
hook_run() { # prints the status, with the hook output in $work/hook.out
    local copy=$tree/deploy/test/mutate-feed-lock.sh
    ( cd "$tree" && LGTM_TEST_POSTGRES=$sentinel "$copy" >"$work/hook.out" 2>&1 )
    echo $?
}
stand_in() { # <status>: a stand-in that records the setting and exits with the status
    cat >"$server_script" <<STANDIN
#!/usr/bin/env bash
printf '%s' "\${LGTM_TEST_POSTGRES-unset}" >"$work/seen"
touch "$work/ran"
exit $1
STANDIN
    chmod +x "$server_script"
    rm -f "$work/seen" "$work/ran"
}

expect "the hook is executable in the repository" test -x "$hook"

rm -f "$server_script"
missing=$(hook_run)
cp "$work/hook.out" "$work/hook.missing"
expect "a missing Server script fails with a nonzero status and a diagnostic naming the script" bash -c '
    [[ $1 -ne 0 && $1 -lt 126 ]] && grep -Fq "check-feed-lock.sh" "$2"
' _ "$missing" "$work/hook.missing"

printf '#!/usr/bin/env bash\ntouch "%s"\nexit 0\n' "$work/ran" >"$server_script"
chmod -x "$server_script"
rm -f "$work/ran"
plain=$(hook_run)
expect "a nonexecutable Server script fails with the same fixed diagnostic and is never run" bash -c '
    [[ $1 -ne 0 && ! -e $2 ]] && cmp -s "$3" "$4" && grep -Fq "check-feed-lock.sh" "$3"
' _ "$plain" "$work/ran" "$work/hook.out" "$work/hook.missing"

for code in 0 1 3 42; do
    stand_in "$code"
    got=$(hook_run)
    expect "the hook returns the exit status $code of the Server script" test "$got" -eq "$code"
    expect "the Server script runs with the inherited setting intact for status $code" bash -c '
        [[ -e $1 && "$(cat "$2")" == "$3" ]]
    ' _ "$work/ran" "$work/seen" "$sentinel"
done
expect "the hook output never shows the setting or its password" bash -c '
    ! grep -Fq -e "sentinel-secret-9d41" -e "sentinel_user" "$1" "$2"
' _ "$work/hook.out" "$work/hook.missing"

printf '%s passed, %s failed\n' "$passed" "$failed"
[[ $failed -eq 0 ]]
