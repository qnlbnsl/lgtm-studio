#!/usr/bin/env bash
# Frozen assertions of work item 6, "Verify first start and restart of the stack".
# Run: bash deploy/test/stack-vault.sh
# Two assertions follow the plan: stack_starts_twice_without_changes and stack_reopens_existing_vault.
# Assumptions the plan leaves open, which the builder must meet:
#  - deploy/up.sh honours COMPOSE_PROJECT_NAME and an absolute LGTM_STACK_STATE_DIR from its environment;
#  - phase lines are exactly "phase <name> start" and "phase <name> done", each on a line of its own;
#  - a failed phase prints no "done" line for itself and no "start" line for a later phase;
#  - the lock is taken on the canonical path of the state folder, so a symbolic link spelling shares it.
source "$(dirname -- "${BASH_SOURCE[0]}")/stack-testlib.sh"

ASSERTIONS=(stack_starts_twice_without_changes stack_reopens_existing_vault)
T=stack_starts_twice_without_changes
O=stack_reopens_existing_vault
READY='postgres and openbao are healthy'
EXPECTED_PHASES='phase database start
phase database done
phase seal start
phase seal done
phase vault start
phase vault done
phase ready start
phase ready done'

has() { grep -Fq -- "$2" <<<"$1"; }
phases() { grep -E '^phase ' <<<"$OUT"; }
last_line() { grep -v '^[[:space:]]*$' <<<"$OUT" | tail -n 1; }
launch_ok() { launch; [[ $RC -eq 0 ]]; }
both_up() { is_healthy postgres && vault_open; }
no_service() { [[ -z $(dc ps -q "$1" 2>/dev/null) ]]; }
state_prints() { # the fingerprints of every durable bootstrap file
    local f
    for f in bao/unseal.key admin/init-response.json server/openbao.json \
        admin/postgres-admin.password server/postgres-app.password bao/postgres-bao.password; do
        printf '%s %s\n' "$f" "$(sfp "$f" | tr '\n' ' ')"
    done
}
all_files() { rootsh 'find /s -type f | sort'; }
digests() { state_prints | awk '{print $1, $2}'; }
export_fns

# ===========================================================================
# Stack one: two launches.
# ===========================================================================
stack_new
check $T "the first launch succeeds" launch_ok
out1=$OUT
check $T "the phases run in order, one start and one done each" test "$(phases)" = "$EXPECTED_PHASES"
check $T "the last line says both daemons are healthy" test "$(last_line)" = "$READY"
check $T "the ready line follows the last phase line" bash -c '
    n=$(grep -n "^phase ready done" <<<"$1" | cut -d: -f1); m=$(grep -nxF "$2" <<<"$1" | cut -d: -f1); [[ -n $n && -n $m && $m -gt $n ]]' _ "$out1" "$READY"
check $T "both daemons are healthy after the first launch" both_up
check $T "the vault answers with PostgreSQL storage and the static seal" bash -c 'bao_status | jq -e ".storage_type == \"postgresql\" and .type == \"static\" and .sealed == false"'
check $T "exactly one secret identifier exists after the first launch" test "$(accessors)" = 1
check $T "the job container does not stay running" no_service openbao-init
check $T "the web and server services are not started by sprint 1" bash -c 'no_service server && no_service web'
before=$(state_prints)
files_before=$(all_files)
check $T "the first launch printed fingerprints for all six files" test "$(grep -c ' [0-9a-f]\{64\} ' <<<"$before")" = 6
check $T "the first launch leaves no temporary file" bash -c '! grep -Eq "tmp|\.new|\.part|~$" <<<"$1"' _ "$files_before"

check $T "the second launch succeeds" launch_ok
out2=$OUT
check $T "the second launch runs the phases in order" test "$(phases)" = "$EXPECTED_PHASES"
check $T "the second launch ends with the ready line" test "$(last_line)" = "$READY"
check $T "both daemons are healthy after the second launch" both_up
check $T "the administrator, seal and server files are unchanged" test "$(state_prints)" = "$before"
check $T "no file appeared or vanished" test "$(all_files)" = "$files_before"
check $T "still exactly one secret identifier" test "$(accessors)" = 1
check $T "the output of neither launch holds the saved administrator token" bash -c '
    t=$(root_token); ! grep -Fq -- "$t" <<<"$1" && ! grep -Fq -- "$t" <<<"$2"' _ "$out1" "$out2"
check $T "the output of neither launch holds the credential" bash -c '
    s=$(scat server/openbao.json | jq -r .secret_id); ! grep -Fq -- "$s" <<<"$1" && ! grep -Fq -- "$s" <<<"$2"' _ "$out1" "$out2"
check $T "the project has one database container and one vault container" bash -c '
    [[ $(docker ps -q --filter "label=com.docker.compose.project=$P" --filter label=com.docker.compose.service=postgres | wc -l) -eq 1 &&
       $(docker ps -q --filter "label=com.docker.compose.project=$P" --filter label=com.docker.compose.service=openbao | wc -l) -eq 1 ]]'

# --- a failed phase stops the launcher with a diagnostic ---
dc stop -t 30 openbao >/dev/null 2>&1
smv bao/unseal.key bao/aside.key
launch
check $T "a missing key fails the launcher" test "$RC" -ne 0
check $T "the diagnostic is printed" has "$OUT" 'seal key missing for existing vault'
check $T "the seal phase started" has "$OUT" 'phase seal start'
check $T "the seal phase did not finish" bash -c '! grep -q "^phase seal done" <<<"$1"' _ "$OUT"
check $T "no later phase started" bash -c '! grep -Eq "^phase (vault|ready) start" <<<"$1"' _ "$OUT"
check $T "the ready line is not printed after a failure" bash -c '! grep -qxF "$2" <<<"$1"' _ "$OUT" "$READY"
check $T "OpenBao is not running without its key" bash -c '! docker ps -q --filter "label=com.docker.compose.project=$P" --filter label=com.docker.compose.service=openbao | grep -q .'
check $T "no new key was created" neg sexists bao/unseal.key
smv bao/aside.key bao/unseal.key
check $T "the launch works again with the key restored" launch_ok
check $T "the files are unchanged after the failed run" test "$(state_prints)" = "$before"

# --- a substituted application password stops the database phase ---
app=server/postgres-app.password
good=$(scat $app)
printf '%s\n' "$(hex64)" | sput $app
launch
check $T "a substituted password fails the launcher" test "$RC" -ne 0
check $T "the database phase did not finish" bash -c '! grep -q "^phase database done" <<<"$1"' _ "$OUT"
check $T "no later phase started after the password failure" bash -c '! grep -Eq "^phase (seal|vault|ready) start" <<<"$1"' _ "$OUT"
check $T "the ready line is not printed after the password failure" bash -c '! grep -qxF "$2" <<<"$1"' _ "$OUT" "$READY"
printf '%s\n' "$good" | sput $app
check $T "the launch works again with the original password" launch_ok
check $T "the content of the files is unchanged after the password failure" test "$(digests)" = "$(awk '{print $1, $2}' <<<"$before")"

# --- the usage guide ---
check $T "the guide exists" test -s "$root/deploy/README.md"
check $T "the guide shows the Compose command with the file" grep -Fq 'docker compose -f deploy/compose.yaml' "$root/deploy/README.md"
check $T "the guide shows ordinary down for shutdown" grep -Eq 'compose.yaml.* down' "$root/deploy/README.md"
check $T "the guide warns against deleting one side of the data" grep -Eqi 'down -v' "$root/deploy/README.md"

# ===========================================================================
# Stack two: two launchers at the same time, one through a link to the state folder.
# ===========================================================================
stack_new
ln -s "$STATE" "$WORK/state-link"
(
    cd "$root" || exit 2
    env LGTM_STACK_STATE_DIR="$STATE" COMPOSE_PROJECT_NAME="$P" timeout 600 deploy/up.sh >"$WORK/c1.out" 2>&1
    echo $? >"$WORK/c1.rc"
) &
j1=$!
(
    cd "$root" || exit 2
    env LGTM_STACK_STATE_DIR="$WORK/state-link" COMPOSE_PROJECT_NAME="$P" timeout 600 deploy/up.sh >"$WORK/c2.out" 2>&1
    echo $? >"$WORK/c2.rc"
) &
j2=$!
wait "$j1" "$j2"
check $T "[concurrent] the first launcher succeeds" test "$(cat "$WORK/c1.rc" 2>/dev/null)" = 0
check $T "[concurrent] the launcher through the link succeeds" test "$(cat "$WORK/c2.rc" 2>/dev/null)" = 0
check $T "[concurrent] both launchers end with the ready line" bash -c '
    [[ $(grep -v "^[[:space:]]*$" "$WORK/c1.out" | tail -n1) == "$1" && $(grep -v "^[[:space:]]*$" "$WORK/c2.out" | tail -n1) == "$1" ]]' _ "$READY"
check $T "[concurrent] one credential and one identifier exist" bash -c '[[ $(accessors) == 1 && $(sstat server/openbao.json) == "10001:10001 400" ]]'
check $T "[concurrent] each launcher printed all eight phase lines once" bash -c '
    [[ $(grep -c "^phase " "$WORK/c1.out") -eq 8 && $(grep -c "^phase " "$WORK/c2.out") -eq 8 ]]'
check $T "[concurrent] both daemons are healthy" both_up

# ===========================================================================
# Stack three: the fixture signing key survives an ordinary restart.
# ===========================================================================
stack_new
if check $O "the first launch succeeds" launch_ok; then
    ROOT=$(root_token)
    sd=$(seed)
    cred_fp=$(sfp server/openbao.json)
    SEED=$sd as_server "$KEY_WRITE" >"$WORK/put.json" 2>&1
    check $O "the fixture key is written as user 10001 with the published file" jq -e '.data.version == 1' "$WORK/put.json"
    check $O "the fixture key reads back before the restart" test "$(as_server "$KEY_READ")" = "$sd"
    rows=$(kv_rows)
    pgvol=$(docker volume ls -q --filter "label=com.docker.compose.project=$P" | sort | tr '\n' ' ')
    check $O "the project has a volume for the database" test -n "$pgvol"
    check $O "ordinary shutdown succeeds" dc down
    check $O "the shutdown removed both daemons" bash -c 'no_service postgres && no_service openbao'
    check $O "the shutdown kept the volume" test "$(docker volume ls -q --filter "label=com.docker.compose.project=$P" | sort | tr '\n' ' ')" = "$pgvol"
    check $O "the shutdown kept the state files" test "$(sstat bao/unseal.key)" = "10002:10002 400"
    check $O "the relaunch succeeds with the same state and volume" launch_ok
    check $O "the relaunch ends with the ready line" test "$(last_line)" = "$READY"
    check $O "the vault is open after the relaunch without an unseal command" vault_open
    check $O "no unseal command appears in the launcher output" bash -c '! grep -Eqi "operator unseal|unseal key" <<<"$1"' _ "$OUT"
    check $O "the same fixture key reads back" test "$(as_server "$KEY_READ")" = "$sd"
    check $O "the published credential is unchanged" test "$(sfp server/openbao.json)" = "$cred_fp"
    check $O "the storage rows are still there" test "$(kv_rows)" -ge "$rows"
    check $O "no second identifier was issued" test "$(accessors)" = 1
    # A restart of the containers without removing them, as after a host restart.
    check $O "a plain restart of both daemons succeeds" dc restart
    check $O "the vault opens again after the plain restart" await 90 vault_open
    check $O "the same fixture key reads back after the plain restart" test "$(as_server "$KEY_READ")" = "$sd"
fi

finish
