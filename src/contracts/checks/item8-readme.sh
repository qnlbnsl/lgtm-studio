#!/bin/bash
# Test of item 8 of the plan go-s0-contract: the README of the contract.
#
# It runs the commands of the plan's table "Tests written first" for item 8 at the
# repository root. Each absence check passes when the text is absent.
# Run it with: bash src/contracts/checks/item8-readme.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$ROOT" || exit 1
R=src/contracts/README.md

[ -f "$R" ] && pass "the README exists" || fail "the README exists"
for w in common.proto link.proto commands.proto events.proto gen/go Lgtm-Link-Token 'Go 1.26' 'link revision 1' LinkService; do
  grep -q -- "$w" "$R" && pass "the README names $w" || fail "the README names $w"
done
for w in 'The calls between the server and the client are not written' 'SECURITY.md' '^- Every call needs a session' '^- Answer a call without a session'; do
  ! grep -q -- "$w" "$R" && pass "the README no longer holds: $w" || fail "the README no longer holds: $w"
done
for w in SessionService.GetServer SessionService.SignIn; do
  grep -q -- "$w" "$R" && pass "the README keeps the exception $w" || fail "the README keeps the exception $w"
done
expect_status "check-private.mjs passes" 0 node scripts/check-private.mjs
finish
