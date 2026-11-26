#!/usr/bin/env bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
server_script=$root/src/server/internal/store/check-feed-lock.sh
if [[ ! -f $server_script || ! -x $server_script ]]; then
    printf '%s\n' 'Server mutation script src/server/internal/store/check-feed-lock.sh is missing or not executable.' >&2
    exit 1
fi

exec "$server_script" "$@"
