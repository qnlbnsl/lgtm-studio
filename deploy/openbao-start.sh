#!/bin/sh
set +x
set -eu

IFS= read -r password < /run/lgtm-db/postgres-bao.password
case "$password" in
    ''|*[!0-9a-f]*) exit 1 ;;
esac
[ "${#password}" -eq 64 ] || exit 1
export BAO_PG_CONNECTION_URL="postgresql://lgtm_bao:$password@postgres:5432/lgtm_bao?sslmode=disable"
exec bao "$@" -config=/openbao/config/lgtm.hcl
