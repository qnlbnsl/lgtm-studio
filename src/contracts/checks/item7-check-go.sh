#!/bin/bash
# Test of item 7 of the plan go-s0-contract: the script scripts/check-go.sh.
#
# It runs the three commands of the plan's table "Tests written first" for item 7.
# The two probes change files, so they run in a scratch copy of the repository
# and never in the checkout that holds this script.
# Run it with: bash src/contracts/checks/item7-check-go.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$ROOT" || exit 1

[ -x scripts/check-go.sh ] && pass "scripts/check-go.sh exists and is executable" || fail "scripts/check-go.sh exists and is executable"

COPY="$(scratch_copy)"
trap 'cd /; git -C "$COPY" status >/dev/null 2>&1 && rm -rf -- "$COPY"' EXIT
cd "$COPY" || exit 1

expect_status "check-go.sh passes on a clean copy" 0 scripts/check-go.sh

# Probe 1: a file that gofmt would change.
printf 'package contracts_test\nfunc  x() {}\n' > src/contracts/zz_probe_test.go
out="$(scripts/check-go.sh 2>&1)"; rc=$?
[ "$rc" -eq 1 ] && pass "a badly formatted file makes check-go.sh exit 1" || fail "a badly formatted file makes check-go.sh exit 1" "exit status $rc"
case "$out" in *src/contracts/zz_probe_test.go*) pass "the output names the badly formatted file";; *) fail "the output names the badly formatted file" "$(printf '%s\n' "$out" | tail -n 8)";; esac
rm -f src/contracts/zz_probe_test.go

# Probe 2: a requirement that go mod tidy would remove.
echo 'require golang.org/x/text v0.20.0' >> go.mod
out="$(scripts/check-go.sh 2>&1)"; rc=$?
[ "$rc" -eq 1 ] && pass "an untidy go.mod makes check-go.sh exit 1" || fail "an untidy go.mod makes check-go.sh exit 1" "exit status $rc"
git checkout -q -- go.mod 2>/dev/null

# The script must pass LGTM_TEST_POSTGRES on and stop at the first failure (statement R10, plan item 7).
if [ -f scripts/check-go.sh ]; then
  head -n 40 scripts/check-go.sh | grep -q 'set -euo pipefail' && pass "check-go.sh starts with set -euo pipefail" || fail "check-go.sh starts with set -euo pipefail"
fi
cd "$ROOT"
finish
