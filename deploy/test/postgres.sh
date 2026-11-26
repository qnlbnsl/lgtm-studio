#!/usr/bin/env bash
# Frozen assertions of work item 2, "Start PostgreSQL and reconcile both service logins".
# Run: bash deploy/test/postgres.sh
# The five assertions follow the plan. Each one prints "assertion <name> pass" or "... fail".
# The script drives deploy/stack-prepare.sh and deploy/compose.yaml in disposable Compose
# projects whose names start with lgtm-s1v-, each with its own state folder.
source "$(dirname -- "${BASH_SOURCE[0]}")/stack-testlib.sh"

ASSERTIONS=(postgres_roles_are_isolated postgres_waits_for_both_databases postgres_has_no_trust_rules
    postgres_repairs_half_initialized_volume postgres_check_driver_authenticates)
A=postgres_roles_are_isolated
W=postgres_waits_for_both_databases
T=postgres_has_no_trust_rules
R=postgres_repairs_half_initialized_volume
D=postgres_check_driver_authenticates

# ---------------------------------------------------------------------------
# Stack A: the waits assertion starts before the roles exist.
# ---------------------------------------------------------------------------
stack_new
A_P=$P A_STATE=$STATE
a_ok=1
check $W "prepare-db succeeds" sp_ok prepare-db || a_ok=0
check $W "the database starts" dc up -d postgres
check $W "the database is not healthy before the roles are reconciled" never_healthy postgres 20
check $W "reconcile-db succeeds" sp_ok reconcile-db || a_ok=0
check $W "the database is healthy after reconciliation" await 120 is_healthy postgres || a_ok=0
[[ -n ${FAILED[$W]:-} ]] && a_ok=0

if [[ $a_ok -ne 1 ]]; then
    for g in $A $D; do check $g "the stack of the first group could not be prepared" false; done
fi

if [[ $a_ok -eq 1 ]]; then
    # --- postgres_roles_are_isolated ---
    declare -A FILEPATH PW
    for r in postgres lgtm_app lgtm_bao; do
        FILEPATH[$r]=$(sfind "$(pw_name "$r")")
        PW[$r]=$(scat "${FILEPATH[$r]}" 2>/dev/null)
        check $A "the file of $r holds 64 lowercase hexadecimal characters and one newline" bash -c '
            [[ $1 =~ ^[0-9a-f]{64}$ && $2 -eq 65 ]]' _ "${PW[$r]}" "$(ssize "${FILEPATH[$r]}")"
    done
    check $A "the three passwords differ" bash -c '[[ $1 != "$2" && $1 != "$3" && $2 != "$3" ]]' _ \
        "${PW[postgres]}" "${PW[lgtm_app]}" "${PW[lgtm_bao]}"
    check $A "the admin file is owned by 70:70 with mode 400" test "$(sstat "${FILEPATH[postgres]}")" = "70:70 400"
    check $A "the application file is owned by 70:10001 with mode 440" test "$(sstat "${FILEPATH[lgtm_app]}")" = "70:10001 440"
    check $A "the vault file is owned by 70:10002 with mode 440" test "$(sstat "${FILEPATH[lgtm_bao]}")" = "70:10002 440"
    for d in bao admin server; do
        check $A "the folder $d is owned by root with mode 700" test "$(sstat "$d")" = "0:0 700"
    done
    check $A "the files lie in the folders admin, server and bao" bash -c '
        [[ $1 == admin/postgres-admin.password && $2 == server/postgres-app.password && $3 == bao/postgres-bao.password ]]' _ \
        "${FILEPATH[postgres]}" "${FILEPATH[lgtm_app]}" "${FILEPATH[lgtm_bao]}"
    check $A "the state folder has mode 700" test "$(rootsh 'stat -c %a /s')" = 700

    check $A "the application login creates a table in its own database" pgq lgtm_app lgtm_app 'create table t_probe(i int)'
    check $A "the vault login creates a table in its own database" pgq lgtm_bao lgtm_bao 'create table t_probe(i int)'
    check $A "the application login cannot open the vault database" bash -c '! pgq lgtm_app lgtm_bao "select 1"' _
    check $A "the vault login cannot open the application database" bash -c '! pgq lgtm_bao lgtm_app "select 1"' _
    for r in lgtm_app lgtm_bao; do
        for db in postgres template1; do
            check $A "$r cannot open the database $db" bash -c '! pgq "$1" "$2" "select 1"' _ "$r" "$db"
        done
    done
    check $A "a wrong password is refused over the network" bash -c '! pgq_with lgtm_app lgtm_app "select 1" "$1"' _ "$(hex64)"
    check $A "each database belongs to its login" test "$(pgq postgres postgres \
        "select string_agg(datname || ':' || pg_get_userbyid(datdba), ',' order by datname) from pg_database where datname in ('lgtm_app','lgtm_bao')")" \
        = "lgtm_app:lgtm_app,lgtm_bao:lgtm_bao"
    check $A "neither login is a superuser or may create databases or roles" test "$(pgq postgres postgres \
        "select string_agg(rolname || ':' || rolsuper::int || rolcreatedb::int || rolcreaterole::int || rolcanlogin::int, ',' order by rolname) from pg_roles where rolname in ('lgtm_app','lgtm_bao')")" \
        = "lgtm_app:0001,lgtm_bao:0001"
    check $A "the file rules hold only valid SCRAM rules" test "$(pgq postgres postgres \
        "select count(*) filter (where error is not null or auth_method is distinct from 'scram-sha-256') || '/' || count(*) from pg_hba_file_rules")" = "0/$(pgq postgres postgres 'select count(*) from pg_hba_file_rules')"
    check $A "the file rules are not empty" test "$(pgq postgres postgres 'select count(*) from pg_hba_file_rules')" -gt 0

    inspect_json postgres >"$WORK/pg.json"
    check $A "the daemon publishes no port" bash -c '
        jq -e "(.[0].HostConfig.PortBindings // {}) | length == 0" "$1" &&
        [[ -z $(docker port "$2" 2>/dev/null) ]]' _ "$WORK/pg.json" "$(cid_of postgres)"
    check $A "the daemon is on the data network only" jq -e --arg n "${P}_data" \
        '.[0].NetworkSettings.Networks | keys == [$n]' "$WORK/pg.json"
    check $A "the data network is internal" test "$(docker network inspect -f '{{.Internal}}' "${P}_data")" = true
    check $A "the daemon runs as 70:70 with every capability dropped and none added" jq -e \
        '.[0].Config.User == "70:70" and ((.[0].HostConfig.CapAdd // []) | length == 0) and ((.[0].HostConfig.CapDrop // []) | index("ALL") != null)' "$WORK/pg.json"
    check $A "the daemon sets no-new-privileges and restarts unless stopped" jq -e \
        '((.[0].HostConfig.SecurityOpt // []) | map(startswith("no-new-privileges")) | any) and .[0].HostConfig.RestartPolicy.Name == "unless-stopped"' "$WORK/pg.json"
    check $A "the effective user and group are 70 and no capability is held" bash -c '
        uid=$(proc_status postgres Uid); gid=$(proc_status postgres Gid); cap=$(proc_status postgres CapEff); nnp=$(proc_status postgres NoNewPrivs)
        [[ $uid == 70*70*70*70 && $gid == 70*70*70*70 && $cap == 0000000000000000 && $nnp == 1 ]]' _
    check $A "the initialization flags and the authentication method are set" jq -e '
        (.[0].Config.Env | map(select(startswith("POSTGRES_INITDB_ARGS="))) | .[0] // "") as $a
        | ($a | contains("--auth-host=scram-sha-256")) and ($a | contains("--auth-local=scram-sha-256"))
          and (.[0].Config.Env | index("POSTGRES_HOST_AUTH_METHOD=scram-sha-256") != null)
          and (.[0].Config.Env | map(startswith("POSTGRES_PASSWORD=")) | any | not)
          and (.[0].Config.Env | map(startswith("POSTGRES_PASSWORD_FILE=")) | any)' "$WORK/pg.json"
    check $A "the three files are mounted under /run/lgtm-db by basename" jq -e '
        [.[0].Mounts[].Destination] as $m
        | ($m | index("/run/lgtm-db/postgres-admin.password") != null)
          and ($m | index("/run/lgtm-db/postgres-app.password") != null)
          and ($m | index("/run/lgtm-db/postgres-bao.password") != null)' "$WORK/pg.json"
    docker logs "$(cid_of postgres)" >"$WORK/pg.log" 2>&1
    check $A "no password appears in the container configuration or the logs" bash -c '
        for p in "$@"; do grep -Fq -- "$p" "$WORK/pg.json" "$WORK/pg.log" && exit 1; done; exit 0' _ \
        "${PW[postgres]}" "${PW[lgtm_app]}" "${PW[lgtm_bao]}"
fi

# --- postgres_check_driver_authenticates, on stack A ---
if [[ $a_ok -eq 1 ]]; then
    driver() {
        (
            cd "$root" || exit 2
            export LGTM_STACK_STATE_DIR="$STATE" COMPOSE_PROJECT_NAME="$P"
            unset POSTGRES_USER POSTGRES_DB
            compose_file=deploy/compose.yaml
            for fn in compose psql_in_stack; do
                body=$(awk -v f="$fn" '$0 ~ "^" f "\\(\\) \\{" {p=1} p {print} p && /^}/ {exit}' deploy/test/checks.sh)
                [[ -n $body ]] || exit 3
                eval "$body"
            done
            psql_in_stack "$@"
        )
    }
    export -f driver
    check $D "the driver function reads the application user and database" test \
        "$(printf '%s' "select current_user || ':' || current_database();" | driver)" = "lgtm_app:lgtm_app"
    check $D "bound arguments and standard input still reach psql" test \
        "$(printf '%s' 'select :n::int + 1;' | driver -v n=6)" = 7
    app_file=$(sfind postgres-app.password)
    good=$(scat "$app_file")
    printf '%s\n' "$(hex64)" | sput "$app_file"
    check $D "a wrong password file makes the driver fail" bash -c '! printf "select 1;" | driver' _
    printf '%s\n' "$good" | sput "$app_file"
    check $D "the original file works again" test \
        "$(printf '%s' "select current_user;" | driver)" = lgtm_app
fi

# --- the waits assertion, second half: substituted passwords on stack A ---
if [[ $a_ok -eq 1 ]]; then
    for r in lgtm_app lgtm_bao; do
        f=$(sfind "$(pw_name "$r")")
        good=$(scat "$f")
        wrong=$(hex64)
        printf '%s\n' "$wrong" | sput "$f"
        check $W "health fails when the mounted password of $r is substituted" await 120 bash -c '! is_healthy postgres' _
        check $W "the login $r cannot use the substituted password" bash -c '! pgq_with "$1" "$1" "select 1" "$2"' _ "$r" "$wrong"
        printf '%s\n' "$good" | sput "$f"
        check $W "health returns with the original password of $r" await 120 is_healthy postgres
        check $W "the login $r works with the original password" pgq "$r" "$r" 'select 1'
    done
fi

# ---------------------------------------------------------------------------
# Stack B: a cluster made without the authentication flags.
# ---------------------------------------------------------------------------
stack_new
check $T "prepare-db succeeds" sp_ok prepare-db
vol=${P}_pgdata
docker volume inspect "$vol" >/dev/null 2>&1 ||
    docker volume create --label "com.docker.compose.project=$P" --label com.docker.compose.volume=pgdata "$vol" >/dev/null
docker run --rm --name "lgtm-s1v-v-$(rnd)" --user 0:0 --network none -v "$vol:/v" --entrypoint sh "$PGIMG" -c 'chown 70:70 /v' >/dev/null 2>&1
admin_file="$STATE/$(sfind postgres-admin.password)"
init_name=lgtm-s1v-i-$(rnd)
docker run -d --name "$init_name" --user 70:70 -e PGDATA=/var/lib/postgresql/data/pgdata \
    -e POSTGRES_PASSWORD_FILE=/pw -v "$vol:/var/lib/postgresql/data" -v "$admin_file:/pw:ro" "$PGIMG" >/dev/null 2>&1
ready_twice() { [[ $(docker logs "$init_name" 2>&1 | grep -c 'ready to accept connections') -ge 2 ]]; }
check $T "the fixture cluster is created without the authentication flags" await 120 ready_twice
docker stop -t 30 "$init_name" >/dev/null 2>&1
docker rm -f -v "$init_name" >/dev/null 2>&1
check $T "the database starts on the old volume" dc up -d postgres
db_answers() { pgq postgres postgres 'select 1'; }
await 90 db_answers
sp reconcile-db
check $T "reconcile-db refuses the cluster" test "$RC" -ne 0
check $T "reconcile-db names the missing authentication rules" bash -c 'grep -Eqi "scram|pg_hba" <<<"$1"' _ "$OUT"
check $T "no login was created before the refusal" test \
    "$(pgq postgres postgres "select count(*) from pg_roles where rolname in ('lgtm_app','lgtm_bao')")" = 0
check $T "no service database was created before the refusal" test \
    "$(pgq postgres postgres "select count(*) from pg_database where datname in ('lgtm_app','lgtm_bao')")" = 0
check $T "the database never becomes healthy" never_healthy postgres 20

# ---------------------------------------------------------------------------
# Stack C: a volume with unfinished role setup.
# ---------------------------------------------------------------------------
stack_new
if check $R "the first setup succeeds" up_db; then
    check $R "the application login stores a row" pgq lgtm_app lgtm_app "create table kept(v text); insert into kept values ('original')"
    app_file=$(sfind postgres-app.password)
    bao_file=$(sfind postgres-bao.password)
    before=$(sfp "$app_file" | head -n 1)
    before_bao=$(sfp "$bao_file" | head -n 1)
    check $R "the vault database is dropped to imitate an interrupted setup" pgq postgres postgres 'drop database lgtm_bao'
    check $R "the vault login is dropped to imitate an interrupted setup" pgq postgres postgres 'drop role lgtm_bao'
    check $R "the stack stops without losing the volume" dc down
    check $R "the setup resumes on the old volume" up_db
    check $R "the application row survived" test "$(pgq lgtm_app lgtm_app 'select v from kept')" = original
    check $R "the application password file is unchanged" test "$(sfp "$app_file" | head -n 1)" = "$before"
    check $R "the application login still opens its database" pgq lgtm_app lgtm_app 'select 1'
    check $R "the vault password file is unchanged" test "$(sfp "$bao_file" | head -n 1)" = "$before_bao"
    check $R "the missing vault login and database were created again" pgq lgtm_bao lgtm_bao 'select 1'
fi

stack_use "$A_P" "$A_STATE"
finish
