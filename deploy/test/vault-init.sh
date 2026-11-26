#!/usr/bin/env bash
# Frozen assertion of work item 4, "Initialize OpenBao and publish one server credential".
# Run: bash deploy/test/vault-init.sh
# The one named assertion is vault_publishes_scoped_credential. The script prints the lines of every
# check as well, so a builder sees which part failed.
# Assumptions the plan leaves open, which the builder must meet:
#  - the job runs as "docker compose --profile setup run --rm -T openbao-init" in the project;
#  - the job prints its fixed diagnostics on either output stream and exits nonzero on refusal;
#  - the job accepts a vault that is open and active, and it never starts the vault itself;
#  - a replacement of server/openbao.json is a rename, so a stale temporary file is never left behind.
source "$(dirname -- "${BASH_SOURCE[0]}")/stack-testlib.sh"

ASSERTIONS=(vault_publishes_scoped_credential)
V=vault_publishes_scoped_credential
UPDATE_KEY='printf "{\"options\":{\"cas\":1},\"data\":{\"private_seed_b64\":\"%s\"}}" "$SEED" | bao write -format=json lgtm/data/server/link/signing-key -'
has() { grep -Fq -- "$2" <<<"$1"; }
job() {
    OUT=$( cd "$root" && envrun timeout 300 docker compose -f deploy/compose.yaml -p "$P" --profile setup run --rm -T openbao-init 2>&1 )
    RC=$?
}
job_ok() { job; [[ $RC -eq 0 ]]; }
role_exists() { bao_run "$(root_token)" read auth/approle/role/lgtm-server; }
login_json() { SEED='' as_server 'printf %s "$login"'; }
vault_up() { up_db && sp_ok seal && dc up -d openbao >/dev/null 2>&1 && await 90 vault_uninitialized; }
listing() { rootsh 'ls -A "/s/$1" | sort | tr "\n" " "' "$1"; }
sec_free() { # <text> <secret>...
    local t=$1 s
    shift
    for s in "$@"; do [[ -n $s ]] && grep -Fq -- "$s" <<<"$t" && return 1; done
    return 0
}

# ===========================================================================
# Stack one: the normal first start.
# ===========================================================================
export_fns
stack_new
S1=$P
if check $V "the vault is up and uninitialized" vault_up; then
    check $V "the first job run succeeds" job_ok
    first_out=$OUT

    resp=admin/init-response.json
    cred=server/openbao.json
    check $V "the response is saved with owner 10002:10002 and mode 400" test "$(sstat $resp)" = "10002:10002 400"
    check $V "the response is the raw initialization output" bash -c '
        scat admin/init-response.json | jq -e "(.root_token | type == \"string\" and length > 0) and (.recovery_keys_b64 | length == 1)"'
    check $V "the published file has owner 10001:10001 and mode 400" test "$(sstat $cred)" = "10001:10001 400"
    check $V "the published file holds exactly two nonempty strings" bash -c '
        scat server/openbao.json | jq -e "keys == [\"role_id\",\"secret_id\"] and (.role_id | type == \"string\" and length > 0) and (.secret_id | type == \"string\" and length > 0)"'
    check $V "the published file is one line with one trailing newline" bash -c '
        [[ $(scat server/openbao.json | wc -l) -eq 1 && $(scat server/openbao.json | tail -c1 | od -An -c | tr -d " ") == "\\n" ]]'
    check $V "the published file is valid UTF-8" bash -c 'scat server/openbao.json | iconv -f UTF-8 -t UTF-8 >/dev/null'
    check $V "no temporary file remains beside the credential" test "$(listing server)" = "openbao.json postgres-app.password "
    check $V "the administrator folder holds only the response and the database password" test "$(listing admin)" = "init-response.json postgres-admin.password "
    check $V "the folders keep owner root and mode 700" bash -c '[[ $(sstat admin) == "0:0 700" && $(sstat server) == "0:0 700" ]]'

    # The login lease and the policy.
    login_json >"$WORK/login.json"
    check $V "the published credential signs in" jq -e '.auth.client_token | length > 0' "$WORK/login.json"
    check $V "the sign-in lease is 900 seconds" jq -e '.auth.lease_duration == 900' "$WORK/login.json"
    check $V "the token carries only the policy lgtm-server" jq -e '.auth.policies == ["lgtm-server"] and .auth.token_policies == ["lgtm-server"]' "$WORK/login.json"
    check $V "the token is a batch token that cannot be renewed" jq -e '(.auth.client_token | startswith("b.") or startswith("hvb.")) and .auth.renewable == false' "$WORK/login.json"

    tok=$(root_token)
    bao_run "$tok" read -format=json auth/approle/role/lgtm-server >"$WORK/role.json" 2>/dev/null
    check $V "the role has the reviewed settings" jq -e '.data
        | .secret_id_ttl == 0 and .secret_id_num_uses == 0 and .token_type == "batch" and .token_ttl == 900
          and .token_max_ttl == 900 and .token_no_default_policy == true and .token_policies == ["lgtm-server"]' "$WORK/role.json"
    bao_run "$tok" policy read lgtm-server >"$WORK/policy.hcl" 2>/dev/null
    check $V "the policy names one path" test "$(grep -c '^[[:space:]]*path[[:space:]]' "$WORK/policy.hcl")" = 1
    check $V "the policy path is the signing key path" grep -Eq '^[[:space:]]*path[[:space:]]+"lgtm/data/server/link/signing-key"' "$WORK/policy.hcl"
    check $V "the policy grants read, create and update only" bash -c '
        c=$(grep -o "\"[a-z]*\"" "$1" | grep -v "signing-key" | tr -d "\"" | sort | tr "\n" " "); [[ $c == "create read update " ]]' _ "$WORK/policy.hcl"
    check $V "the policy holds no wildcard" bash -c '! grep -Eq "[*+]|sudo|delete|list|deny" "$1"' _ "$WORK/policy.hcl"
    bao_run "$tok" secrets list -format=json >"$WORK/mounts.json" 2>/dev/null
    check $V "the engine at lgtm is key-value version 2" jq -e '.["lgtm/"].type == "kv" and .["lgtm/"].options.version == "2"' "$WORK/mounts.json"
    bao_run "$tok" auth list -format=json >"$WORK/auths.json" 2>/dev/null
    check $V "AppRole is enabled at approle" jq -e '.["approle/"].type == "approle"' "$WORK/auths.json"
    check $V "exactly one secret identifier exists" test "$(accessors)" = 1

    # The signing key round trip, and the denials.
    sd=$(seed)
    SEED=$sd as_server "$KEY_WRITE" >"$WORK/w1.json" 2>&1
    check $V "the server writes its signing key with cas 0" jq -e '.data.version == 1' "$WORK/w1.json"
    check $V "the server reads the same seed back" test "$(as_server "$KEY_READ")" = "$sd"
    write_new() { SEED=$(seed) as_server "$KEY_WRITE"; }
    write_update() { SEED=$(seed) as_server "$UPDATE_KEY"; }
    check $V "a second creation with cas 0 is refused" neg write_new
    check $V "the first value survives the refused write" test "$(as_server "$KEY_READ")" = "$sd"
    check $V "the server may update the key with the right version" write_update

    denied() { neg as_server "$1"; }
    check $V "a sibling path under link cannot be read" denied 'bao read lgtm/data/server/link/other'
    check $V "a sibling path under link cannot be written" denied 'bao write lgtm/data/server/link/other v=1'
    check $V "another server path cannot be read" denied 'bao read lgtm/data/server/other'
    check $V "the key metadata cannot be read" denied 'bao read lgtm/metadata/server/link/signing-key'
    check $V "the key cannot be deleted" denied 'bao delete lgtm/data/server/link/signing-key'
    check $V "the policy cannot be read" denied 'bao read sys/policies/acl/lgtm-server'
    check $V "policies cannot be listed" denied 'bao list sys/policies/acl'
    check $V "the mounts cannot be listed" denied 'bao secrets list'
    check $V "the role cannot be read" denied 'bao read auth/approle/role/lgtm-server'
    check $V "the role identifier cannot be read" denied 'bao read auth/approle/role/lgtm-server/role-id'
    check $V "a new secret identifier cannot be issued" denied 'bao write -f auth/approle/role/lgtm-server/secret-id'
    check $V "identifiers cannot be listed" denied 'bao list auth/approle/role/lgtm-server/secret-id'
    check $V "a token cannot be created" denied 'bao token create -policy=default'
    check $V "the seal status cannot be changed" denied 'bao operator seal'
    check $V "the audit devices cannot be listed" denied 'bao audit list'
    check $V "without a token the policy cannot be read" neg bao_run '' read sys/policies/acl/lgtm-server
    check $V "without a token the mounts cannot be listed" neg bao_run '' secrets list
    check $V "without a token the signing key cannot be read" neg bao_run '' read lgtm/data/server/link/signing-key
    check $V "without a token initialization is refused" neg bao_run '' operator init
    check $V "the administrator token is not accepted as the published file" neg bao_run "not-a-token" read lgtm/data/server/link/signing-key

    # Secret handling.
    check $V "the job output holds no secret" sec_free "$first_out" "$tok" "$(jq -r .recovery_keys_b64[0] <<<"$(scat $resp)")" \
        "$(scat $cred | jq -r .secret_id)" "$(scat $cred | jq -r .role_id)"
    docker logs "$(cid_of openbao)" >"$WORK/bao.log" 2>&1
    check $V "the daemon log holds no root token or secret identifier" bash -c '
        ! grep -Fq -- "$1" "$WORK/bao.log" && ! grep -Fq -- "$2" "$WORK/bao.log"' _ "$tok" "$(scat $cred | jq -r .secret_id)"
    check $V "the database daemon log holds no root token" bash -c '
        dc logs postgres 2>&1 | grep -Fq -- "$1" && exit 1; exit 0' _ "$tok"
    compose_config >"$WORK/cfg.json"
    check $V "the job service is in the setup profile" jq -e '.services["openbao-init"].profiles | index("setup") != null' "$WORK/cfg.json"
    check $V "the job uses the pinned daemon image" jq -e --arg i "$BAOIMG" '.services["openbao-init"].image == $i' "$WORK/cfg.json"
    check $V "the job joins the vault network only" jq -e '(.services["openbao-init"].networks // {} | keys) == ["vault"]' "$WORK/cfg.json"
    check $V "the job runs as root and drops all capabilities" jq -e '
        .services["openbao-init"] | .user == "0:0" and ((.cap_drop // []) | index("ALL") != null)
        and ((.cap_add // []) | sort | map(select(. != "CHOWN" and . != "DAC_OVERRIDE")) | length == 0)
        and ((.security_opt // []) | map(startswith("no-new-privileges")) | any)' "$WORK/cfg.json"
    check $V "the job mounts only admin, server and the read-only policy" jq -e '
        [.services["openbao-init"].volumes[] | .source | split("/") | last] | sort == ["admin","openbao-server-policy.hcl","server"]' "$WORK/cfg.json"
    check $V "the policy mount is read-only" jq -e '
        [.services["openbao-init"].volumes[] | select(.target | test("policy")) | .read_only] == [true]' "$WORK/cfg.json"
    check $V "the job never mounts the seal key or the database folder" jq -e '
        [.services["openbao-init"].volumes[] | .source] | map(test("/bao(/|$)")) | any | not' "$WORK/cfg.json"
    check $V "the job has no secret in its environment" jq -e '
        [(.services["openbao-init"].environment // {}) | to_entries[] | select(.key | test("TOKEN|SECRET|PASSWORD|KEY"; "i"))] | length == 0' "$WORK/cfg.json"
    check $V "the job publishes no port" jq -e '((.services["openbao-init"].ports // []) | length) == 0' "$WORK/cfg.json"
    check $V "the server service never mounts the administrator folder" jq -e '
        [(.services.server.volumes // [])[] | .source // ""] | map(test("/admin(/|$)")) | any | not' "$WORK/cfg.json"
    check $V "the job script never calls jq" bash -c '! grep -Eq "(^|[^A-Za-z_./-])jq([^A-Za-z_]|$)" deploy/openbao-init.sh'
    check $V "the job script does not trace its commands" bash -c '! grep -Eq "set[[:space:]]+-[A-Za-z]*x|xtrace" deploy/openbao-init.sh'
    check $V "the job script exports the token to no later process" bash -c '! grep -Eq "export[[:space:]]+BAO_TOKEN" deploy/openbao-init.sh'
    check $V "the pinned image has no jq" neg docker run --rm --name "lgtm-s1v-j-$(rnd)" --network none --entrypoint sh "$BAOIMG" -c 'command -v jq'

    # A repeat leaves everything in place.
    fp_resp=$(sfp $resp)
    fp_cred=$(sfp $cred)
    acc=$(accessors)
    check $V "the repeat job succeeds" job_ok
    check $V "the repeat leaves the response untouched" test "$(sfp $resp)" = "$fp_resp"
    check $V "the repeat leaves the published file untouched" test "$(sfp $cred)" = "$fp_cred"
    check $V "the repeat issues no second identifier" test "$(accessors)" = "$acc"
    check $V "the repeat leaves the role in place" bash -c 'role_exists'
    check $V "the published file still signs in after the repeat" bash -c 'login_json | jq -e ".auth.client_token | length > 0"'
    check $V "the repeat leaves no temporary file" test "$(listing server)" = "openbao.json postgres-app.password "

    # An unusable final file.
    printf '{"role_id":"x","secret_id":"y"}\n' | sput $cred
    fp_bad=$(sfp $cred)
    check $V "an unusable credential makes the job fail" neg job_ok
    check $V "the diagnostic asks for repair" has "$OUT" 'server vault credential needs repair'
    check $V "the unusable file is not rewritten" test "$(sfp $cred)" = "$fp_bad"
    check $V "no second identifier appears for an unusable file" test "$(accessors)" = "$acc"
    check $V "the response is still untouched" test "$(sfp $resp)" = "$fp_resp"

    # An identifier exists, the final file is gone.
    srm $cred
    check $V "an existing identifier without the final file makes the job fail" neg job_ok
    check $V "the diagnostic names the incomplete state" has "$OUT" 'vault is initialized but bootstrap state is incomplete'
    check $V "no credential is published in that state" neg sexists $cred
    check $V "no identifier is added in that state" test "$(accessors)" = "$acc"
    check $V "the response survives the refusal" test "$(sfp $resp)" = "$fp_resp"
fi

# ===========================================================================
# Stack two: initialized by hand ahead of the job, with a truncated response first.
# ===========================================================================
stack_new
if check $V "[truncated] the vault is up and uninitialized" vault_up; then
    init_vault
    rows=$(kv_rows)
    sed '$d' "$WORK/init.json" | sput admin/init-response.json
    sown admin/init-response.json 10002:10002 400
    fp_resp=$(sfp admin/init-response.json)
    check $V "[truncated] the fixture still holds the token" bash -c 'scat admin/init-response.json | grep -Fq "$(jq -r .root_token "$WORK/init.json")"'
    check $V "[truncated] the fixture is no longer valid JSON" neg bash -c 'scat admin/init-response.json | jq -e . >/dev/null'
    check $V "[truncated] a truncated response makes the job fail" neg job_ok
    check $V "[truncated] the diagnostic names the incomplete state" has "$OUT" 'vault is initialized but bootstrap state is incomplete'
    check $V "[truncated] no credential is published" neg sexists server/openbao.json
    check $V "[truncated] the response is preserved" test "$(sfp admin/init-response.json)" = "$fp_resp"
    check $V "[truncated] the vault is still initialized and open" vault_open
    check $V "[truncated] no role was created" neg bao_run "$ROOT" read auth/approle/role/lgtm-server
    check $V "[truncated] the storage rows are intact" test "$(kv_rows)" -ge "$rows"
    check $V "[truncated] the job output holds no token" sec_free "$OUT" "$ROOT"
    # Restore the complete response: the job now succeeds on the vault initialized by hand.
    cp "$WORK/init.json" "$WORK/init.full.json"
    sput admin/init-response.json <"$WORK/init.full.json"
    sown admin/init-response.json 10002:10002 400
    check $V "[repair] the job succeeds with the complete response" job_ok
    check $V "[repair] one credential is published" test "$(sstat server/openbao.json)" = "10001:10001 400"
    check $V "[repair] exactly one identifier exists" test "$(accessors)" = 1
    check $V "[repair] the published file signs in" bash -c 'login_json | jq -e ".auth.lease_duration == 900"'
    check $V "[repair] the response is the one saved before" test "$(scat admin/init-response.json | sha256sum)" = "$(sha256sum <"$WORK/init.full.json")"
fi

# ===========================================================================
# Stack three: the state is ahead of the database, and a missing response.
# ===========================================================================
for ev in admin/init-response.json server/openbao.json; do
    stack_new
    if check $V "[ahead $ev] the vault is up and uninitialized" vault_up; then
        printf '{"root_token":"placeholder"}\n' | sput "$ev"
        fp=$(sfp "$ev")
        check $V "[ahead $ev] the job refuses" neg job_ok
        check $V "[ahead $ev] the diagnostic says the state is ahead" has "$OUT" 'state directory is ahead of the database'
        check $V "[ahead $ev] the file is preserved" test "$(sfp "$ev")" = "$fp"
        check $V "[ahead $ev] the vault is still uninitialized" vault_uninitialized
        check $V "[ahead $ev] no other bootstrap file appears" test "$(rootsh 'find /s/admin /s/server -type f | wc -l')" = 3
    fi
done

stack_new
if check $V "[no response] the vault is up and uninitialized" vault_up; then
    init_vault
    rows=$(kv_rows)
    check $V "[no response] a vault initialized without a saved response makes the job fail" neg job_ok
    check $V "[no response] the diagnostic names the incomplete state" has "$OUT" 'vault is initialized but bootstrap state is incomplete'
    check $V "[no response] no response file is invented" neg sexists admin/init-response.json
    check $V "[no response] no credential is published" neg sexists server/openbao.json
    check $V "[no response] the database is intact" test "$(kv_rows)" -ge "$rows"
    check $V "[no response] the vault remains open" vault_open
fi

finish
