#!/usr/bin/env bash
# Frozen assertions of work item 3, "Prepare the key and start OpenBao".
# Run: bash deploy/test/seal.sh
# Four assertions follow the plan: seal_key_is_created_once, missing_key_preserves_existing_vault,
# wrong_key_preserves_existing_vault and vault_has_no_host_port.
# Assumptions the plan leaves open, which the builder must meet:
#  - the key file lies at bao/unseal.key under the state folder and holds 32 raw bytes;
#  - "stack-prepare.sh seal" prints its fixed diagnostic on either output stream and exits nonzero;
#  - local bootstrap evidence means the file admin/init-response.json or server/openbao.json.
source "$(dirname -- "${BASH_SOURCE[0]}")/stack-testlib.sh"

ASSERTIONS=(seal_key_is_created_once missing_key_preserves_existing_vault wrong_key_preserves_existing_vault vault_has_no_host_port seal_stops_when_storage_cannot_be_checked)
C=seal_key_is_created_once
M=missing_key_preserves_existing_vault
G=wrong_key_preserves_existing_vault
N=vault_has_no_host_port
D=seal_stops_when_storage_cannot_be_checked
KEY=bao/unseal.key

never_open() { # <seconds>
    local deadline=$((SECONDS + $1))
    while ((SECONDS < deadline)); do
        vault_open && return 1
        sleep 2
    done
    return 0
}
has() { grep -Fq -- "$2" <<<"$1"; }

export_fns
stack_new
base_ok=1
check $C "the database part succeeds" up_db || base_ok=0
check $C "seal succeeds on the first run" sp_ok seal || base_ok=0
if ((base_ok)); then
    check $C "the key is owned by 10002:10002 with mode 400" test "$(sstat $KEY)" = "10002:10002 400"
    check $C "the key holds 32 bytes" test "$(ssize $KEY)" = 32
    check $C "the key is a regular file" rootsh 'test -f /s/bao/unseal.key && ! test -L /s/bao/unseal.key'
    k1=$(sfp $KEY)
    check $C "the key does not equal zero bytes" bash -c '! rootsh "head -c32 /s/bao/unseal.key | tr -d \"\\0\" | wc -c | grep -qx 0"'
    check $C "a second seal succeeds" sp_ok seal
    check $C "the key is unchanged by the second run" test "$(sfp $KEY)" = "$k1"
    check $C "a third seal succeeds" sp_ok seal
    check $C "the key is still unchanged" test "$(sfp $KEY)" = "$k1"
    check $C "no other key file appears" test "$(rootsh 'ls /s/bao' | sort | tr '\n' ' ')" = "postgres-bao.password unseal.key "
    check $C "the folder bao keeps owner root and mode 700" test "$(sstat bao)" = "0:0 700"

    check $C "OpenBao starts" dc up -d openbao
    check $C "the vault starts uninitialized and sealed state is reported" await 90 vault_uninitialized
    check $C "the daemon reads the key from its fixed path" bash -c '
        dc exec -T openbao sh -c "test -r /run/lgtm-bao/unseal.key && ! test -w /run/lgtm-bao/unseal.key"'
    check $C "the key mount is read-only" bash -c '
        docker inspect "$(cid_of openbao)" | jq -e "[.[0].Mounts[] | select(.Destination==\"/run/lgtm-bao/unseal.key\" or .Destination==\"/run/lgtm-bao\")] | length > 0 and all(.RW == false)"'
    init_vault
    check $C "the vault is initialized and open without a manual unseal" vault_open
    put_sample orig
    check $M "the fixture vault holds a record" test "$(get_sample)" = orig
    rows=$(kv_rows)
    check $M "the storage holds rows" test "${rows:-0}" -gt 0
fi

# --- the missing key, with vault rows ---
if ((base_ok)) && [[ -n ${ROOT:-} ]]; then
    dc stop -t 30 openbao >/dev/null 2>&1
    check $M "the key is moved aside" smv $KEY bao/aside.key
    sp seal
    check $M "seal refuses without the key" test "$RC" -ne 0
    check $M "seal names the missing key of an existing vault" has "$OUT" 'seal key missing for existing vault'
    check $M "no new key was created" neg sexists $KEY
    check $M "the moved key is untouched" test "$(sfp1 bao/aside.key)" = "$(echo "$k1" | head -n 1)"
    check $M "the rows are preserved" test "$(kv_rows)" = "$rows"
    check $M "the original key is put back" smv bao/aside.key $KEY
    check $M "seal accepts the original key" sp_ok seal
    check $M "the key bytes are unchanged" test "$(sfp1 $KEY)" = "$(echo "$k1" | head -n 1)"
    dc up -d openbao >/dev/null 2>&1
    check $M "the original key opens the record again" await 90 vault_open
    check $M "the original record reads back" test "$(get_sample)" = orig
fi

# --- a wrong key of valid length and a malformed key, with vault rows ---
if ((base_ok)) && [[ -n ${ROOT:-} ]]; then
    dc stop -t 30 openbao >/dev/null 2>&1
    smv $KEY bao/aside.key
    head -c32 /dev/urandom | sput $KEY
    sown $KEY 10002:10002 400
    sp seal
    check $G "seal does not replace a key of valid length" test "$(ssize $KEY)" = 32
    check $G "the different key is not the original" test "$(sfp1 $KEY)" != "$(echo "$k1" | head -n 1)"
    dc up -d openbao >/dev/null 2>&1
    check $G "the vault stays sealed with a different key" never_open 30
    check $G "the vault is still initialized and sealed" vault_sealed
    check $G "the rows are preserved with a different key" test "$(kv_rows)" = "$rows"
    dc stop -t 30 openbao >/dev/null 2>&1
    smv bao/aside.key $KEY
    dc up -d openbao >/dev/null 2>&1
    check $G "the original key opens the vault again" await 90 vault_open
    check $G "the original record reads back after the wrong key" test "$(get_sample)" = orig

    dc stop -t 30 openbao >/dev/null 2>&1
    smv $KEY bao/aside.key
    printf 'short' | sput $KEY
    sown $KEY 10002:10002 400
    sp seal
    check $G "seal refuses a malformed key" test "$RC" -ne 0
    check $G "seal reports an invalid key" has "$OUT" 'seal key invalid'
    check $G "the malformed file is not replaced" test "$(scat $KEY)" = short
    check $G "the rows are preserved with a malformed key" test "$(kv_rows)" = "$rows"
    smv bao/aside.key bao/aside2.key 2>/dev/null
    rootsh 'rm -f /s/bao/unseal.key; ln -s /s/bao/aside2.key /s/bao/unseal.key; true'
    sp seal
    check $G "seal refuses a symbolic link as the key" test "$RC" -ne 0
    check $G "seal reports an invalid key for the link" has "$OUT" 'seal key invalid'
    rootsh 'rm -f /s/bao/unseal.key; mv /s/bao/aside2.key /s/bao/unseal.key; true'
    check $G "the original key is restored" test "$(sfp1 $KEY)" = "$(echo "$k1" | head -n 1)"
    dc up -d openbao >/dev/null 2>&1
    check $G "the original key opens the vault after the malformed run" await 90 vault_open
    check $G "the original record still reads back" test "$(get_sample)" = orig
fi

# --- no host port, identities, storage, networks (stack one) ---
if ((base_ok)) && [[ -n ${ROOT:-} ]]; then
    compose_config >"$WORK/config.json"
    check $N "the Compose file renders" jq -e '.services | length > 0' "$WORK/config.json"
    check $N "no service declares a port" jq -e '[.services[] | select(.ports != null and (.ports | length) > 0)] | length == 0' "$WORK/config.json"
    check $N "every service drops all capabilities and sets no-new-privileges" jq -e '
        [.services[] | select(((.cap_drop // []) | index("ALL")) == null or ((.security_opt // []) | map(startswith("no-new-privileges")) | any | not))] | length == 0' "$WORK/config.json"
    check $N "every service names its numeric user" jq -e '[.services[] | select((.user // "") | test("^[0-9]+:[0-9]+$") | not)] | length == 0' "$WORK/config.json"
    check $N "only the allowed capabilities are ever added" jq -e '
        [.services[] | (.cap_add // [])[] | select(. != "CHOWN" and . != "DAC_OVERRIDE" and . != "FOWNER")] | length == 0' "$WORK/config.json"
    check $N "a service that adds capabilities runs as root and has no data network" jq -e '
        [.services[] | select((.cap_add // []) | length > 0) | select(.user != "0:0" or ((.networks // {}) | has("data")))] | length == 0' "$WORK/config.json"
    check $N "every file bind sets create_host_path to false" jq -e '
        [.services[] | (.volumes // [])[] | select(.type == "bind") | select(.bind.create_host_path != false)] | length == 0' "$WORK/config.json"
    check $N "the networks data and vault are internal" jq -e '.networks.data.internal == true and .networks.vault.internal == true' "$WORK/config.json"
    check $N "the daemon service uses the pinned image" jq -e --arg i "${BAOIMG}" '.services.openbao.image == $i' "$WORK/config.json"
    check $N "the daemon service uses the explicit server entrypoint" jq -e '
        (.services.openbao.entrypoint // []) == ["/bin/sh","/openbao-start.sh","server"]
        or ((.services.openbao.command // []) == ["server"] and (.services.openbao.entrypoint // []) == ["/bin/sh","/openbao-start.sh"])' "$WORK/config.json"
    check $N "the daemon is declared as 10002:10002" jq -e '.services.openbao.user == "10002:10002"' "$WORK/config.json"
    check $N "the database is declared as 70:70" jq -e '.services.postgres.user == "70:70"' "$WORK/config.json"
    check $N "the daemon depends on a healthy database" jq -e '.services.openbao.depends_on.postgres.condition == "service_healthy"' "$WORK/config.json"
    check $N "the project volume pgdata exists" jq -e '.volumes | has("pgdata")' "$WORK/config.json"

    inspect_json openbao >"$WORK/bao.json"
    check $N "the daemon container publishes no port" bash -c '
        jq -e "(.[0].HostConfig.PortBindings // {}) | length == 0" "$1" && [[ -z $(docker port "$2" 2>/dev/null) ]]' _ "$WORK/bao.json" "$(cid_of openbao)"
    check $N "the daemon is on the data and vault networks only" jq -e --arg d "${P}_data" --arg v "${P}_vault" \
        '(.[0].NetworkSettings.Networks | keys | sort) == ([$d,$v] | sort)' "$WORK/bao.json"
    check $N "the vault network is internal" test "$(docker network inspect -f '{{.Internal}}' "${P}_vault")" = true
    check $N "the data network is internal" test "$(docker network inspect -f '{{.Internal}}' "${P}_data")" = true
    check $N "the daemon runs as 10002:10002 with every capability dropped" jq -e \
        '.[0].Config.User == "10002:10002" and ((.[0].HostConfig.CapAdd // []) | length == 0) and ((.[0].HostConfig.CapDrop // []) | index("ALL") != null)' "$WORK/bao.json"
    check $N "the daemon is not in privileged mode" jq -e '.[0].HostConfig.Privileged == false' "$WORK/bao.json"
    check $N "the effective identity of the daemon is 10002 with no capability" bash -c '
        [[ $(proc_status openbao Uid) == 10002*10002*10002*10002 && $(proc_status openbao Gid) == 10002*10002*10002*10002 &&
           $(proc_status openbao CapEff) == 0000000000000000 && $(proc_status openbao NoNewPrivs) == 1 ]]'
    check $N "the daemon uses PostgreSQL storage" bash -c 'bao_status | jq -e ".storage_type == \"postgresql\""'
    check $N "the daemon uses the static seal and not development mode" bash -c 'bao_status | jq -e ".type == \"static\""'
    check $N "the daemon container publishes no port in its configuration" bash -c '
        jq -e "(.[0].HostConfig.PortBindings // {}) | length == 0" <(inspect_json openbao)'
    check $N "Docker reports no published port of the daemon container" bash -c '
        [[ -n $(cid_of openbao) && -z $(docker port "$(cid_of openbao)") ]]'
    check $N "the database daemon publishes no port in its configuration" bash -c '
        jq -e "(.[0].HostConfig.PortBindings // {}) | length == 0" <(inspect_json postgres)'
    check $N "the key does not appear in the configuration of the daemon" bash -c '
        ! docker inspect "$(cid_of openbao)" | grep -Fq "$(rootsh "od -An -tx1 /s/bao/unseal.key | tr -d \" \\n\"")"'
fi

# --- second stack: bootstrap evidence without vault rows ---
for ev in admin/init-response.json server/openbao.json; do
    stack_new
    if check $M "[$ev] the database part succeeds" up_db && check $M "[$ev] the first seal succeeds" sp_ok seal; then
        ksum=$(sfp1 $KEY)
        check $M "[$ev] the table of the vault holds no rows" test "$(kv_rows)" = 0
        printf '{"root_token":"placeholder"}\n' | sput "$ev"
        smv $KEY bao/aside.key
        sp seal
        check $M "[$ev] seal refuses without the key" test "$RC" -ne 0
        check $M "[$ev] seal names the missing key of an existing vault" has "$OUT" 'seal key missing for existing vault'
        check $M "[$ev] no new key was created" neg sexists $KEY
        check $M "[$ev] the evidence file is untouched" test "$(scat "$ev")" = '{"root_token":"placeholder"}'
        smv bao/aside.key $KEY
        check $M "[$ev] the original key is accepted afterwards" sp_ok seal
        check $M "[$ev] the key bytes are unchanged" test "$(sfp1 $KEY)" = "$ksum"
    fi
done

# --- third stack: no vault data and no evidence: a missing key is created ---
stack_new
if check $C "[fresh] the database part succeeds" up_db && check $C "[fresh] the first seal succeeds" sp_ok seal; then
    smv $KEY bao/aside.key
    check $C "[fresh] a fresh installation creates a new key" sp_ok seal
    check $C "[fresh] the new key has the right owner, mode and size" bash -c '[[ $(sstat bao/unseal.key) == "10002:10002 400" && $(ssize bao/unseal.key) == 32 ]]'
fi

# --- fourth stack: a failed probe of the storage must stop seal and must not write a key ---
# The key is missing and no bootstrap evidence exists, so only the probe tells an old vault from a new one.
stack_new
if check $D "the database part succeeds" up_db && check $D "the first seal succeeds" sp_ok seal; then
    srm $KEY
    dc stop -t 30 postgres >/dev/null 2>&1
    sp seal
    check $D "[database stopped] seal refuses" test "$RC" -ne 0
    check $D "[database stopped] seal names the storage check" has "$OUT" 'Could not check OpenBao storage in PostgreSQL.'
    check $D "[database stopped] no key was written" neg sexists $KEY
    dc up -d postgres >/dev/null 2>&1
    check $D "the database returns to health" await 120 is_healthy postgres
    scat bao/postgres-bao.password >"$WORK/bao-pw.orig"
    hex64 | sput bao/postgres-bao.password
    sp seal
    check $D "[wrong password] seal refuses" test "$RC" -ne 0
    check $D "[wrong password] seal names the storage check" has "$OUT" 'Could not check OpenBao storage in PostgreSQL.'
    check $D "[wrong password] no key was written" neg sexists $KEY
    sput bao/postgres-bao.password <"$WORK/bao-pw.orig"
    check $D "the original password is restored" test "$(scat bao/postgres-bao.password)" = "$(cat "$WORK/bao-pw.orig")"
    check $D "with the database asked again, a fresh installation gets its key" sp_ok seal
    check $D "the new key has the right owner, mode and size" bash -c '[[ $(sstat bao/unseal.key) == "10002:10002 400" && $(ssize bao/unseal.key) == 32 ]]'
fi

finish
