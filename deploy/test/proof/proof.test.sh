#!/usr/bin/env bash
# Frozen assertions of work item 1, "Prove the pinned images".
#
# The test runs `proof.sh` from this folder and reads what it prints. The
# contract between the two files is the following output, one fact per line:
#
#   assertion <name> pass
#   image <service> <os/arch> <repository digest> user=<user> entrypoint=<json> cmd=<json>
#   rows before=<count> after=<count>
#   cleanup=true
#
# The assertion names are postgres_storage_round_trip, static_seal_restart,
# missing_seal_key_blocks_unseal and wrong_seal_key_stays_sealed. The service
# names are postgres and openbao. The script exits zero only when all four
# assertions pass and the teardown is complete. The test also reads `bao.hcl`
# directly, so a proof that never configured the production shape fails here.
set -u
set -o pipefail
export LC_ALL=C

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
proof=$here/proof.sh
config=$here/bao.hcl
pg_digest=sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d
bao_digest=sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9
assertions=(postgres_storage_round_trip static_seal_restart missing_seal_key_blocks_unseal wrong_seal_key_stays_sealed)

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

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
out=$scratch/out
err=$scratch/err

# The proof labels every resource with lgtm.s1v.fixture=<its project>, and its project name is
# lgtm-s1v-proof-<random>. Only resources of such a project that appear during this run count, so the
# stacks of other agents (lgtm-s1v-test-*, lgtm-s1v-<hex>) never change the result.
leftovers() {
    {
        docker ps -a --filter 'label=lgtm.s1v.fixture' --format '{{.Names}}'
        docker volume ls --filter 'label=lgtm.s1v.fixture' --format '{{.Name}}'
        docker network ls --filter 'label=lgtm.s1v.fixture' --format '{{.Name}}'
    } 2>/dev/null | grep -E '^lgtm-s1v-proof-' | sort || true
}
new_leftovers() { comm -13 <(printf '%s\n' "$1") <(printf '%s\n' "$2"); }

before=$(leftovers)
status=0
if [[ -x $proof ]]; then
    "$proof" >"$out" 2>"$err" || status=$?
else
    status=127
    : >"$out"
    : >"$err"
fi
after=$(leftovers)

expect "the proof script exists, is executable and exits zero" test "$status" -eq 0
for name in "${assertions[@]}"; do
    expect "assertion $name passes" grep -Fxq "assertion $name pass" "$out"
    expect "assertion $name is reported once and never as a failure" test \
        "$(grep -Ec "^assertion $name( |$)" "$out")" -eq 1
done

expect "the output ends with cleanup=true" test "$(tail -n 1 "$out")" = "cleanup=true"
expect "no container, volume or network of the proof remains" test -z "$(new_leftovers "$before" "$after")"

expect "the PostgreSQL image is pinned by the exact digest" \
    grep -Eq "^image postgres .*postgres.*@$pg_digest" "$out"
expect "the OpenBao image is pinned by the exact digest" \
    grep -Eq "^image openbao .*openbao.*@$bao_digest" "$out"
expect "the PostgreSQL image runs as user 70 and the platform is recorded" \
    grep -Eq '^image postgres linux/[a-z0-9]+ .* user=(70|70:70|postgres)( |$)' "$out"
expect "the OpenBao platform is recorded" \
    grep -Eq '^image openbao linux/[a-z0-9]+ ' "$out"

# The row growth proves that the vault wrote to PostgreSQL and not to memory.
expect "the storage row count grew from absent or zero to a positive number" bash -c '
    line=$(grep -E "^rows before=[0-9]+ after=[0-9]+$" "$1" | head -n 1) || exit 1
    before=${line#rows before=}; before=${before%% *}
    after=${line##*after=}
    [[ $after -gt $before && $after -gt 0 ]]
' _ "$out"

# Nothing secret may reach the console.
expect "the output holds no token, key or password value" bash -c '
    ! grep -Eqi "(root_token|hvs\.|s\.[A-Za-z0-9]{20,}|unseal_keys|password=)" "$1" "$2"
' _ "$out" "$err"

# The configuration of the production shape.
expect "bao.hcl selects the PostgreSQL storage backend" \
    grep -Eq '^[[:space:]]*storage[[:space:]]+"postgresql"' "$config"
expect "bao.hcl selects the static seal with key identifier lgtm-1" bash -c '
    grep -Eq "^[[:space:]]*seal[[:space:]]+\"static\"" "$1" &&
    grep -Eq "current_key_id[[:space:]]*=[[:space:]]*\"lgtm-1\"" "$1"
' _ "$config"
expect "bao.hcl reads the key from /run/lgtm-bao/unseal.key" \
    grep -Fq '/run/lgtm-bao/unseal.key' "$config"
expect "bao.hcl connects with sslmode=disable and not as postgres" bash -c '
    grep -Fq "sslmode=disable" "$1" && ! grep -Eq "//postgres[:@]" "$1"
' _ "$config"
expect "bao.hcl does not use in-memory storage or development mode" bash -c '
    ! grep -Eq "storage[[:space:]]+\"(inmem|file|raft)\"|dev_root_token|-dev" "$1"
' _ "$config"
expect "bao.hcl holds no literal password" bash -c '
    ! grep -Eq "postgres(ql)?://[^:@/ ]+:[^@ ]+@" "$1"
' _ "$config"

printf '%s passed, %s failed\n' "$passed" "$failed"
[[ $failed -eq 0 ]]
