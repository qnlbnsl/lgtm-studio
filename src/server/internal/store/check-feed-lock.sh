#!/usr/bin/env bash
# This script proves that the feed ordering test fails when the feed lock is removed.
# It tests an archive of the committed HEAD, so the checkout must have no changes under src, go.mod or go.sum.
# Run it from the repository root with LGTM_TEST_POSTGRES set, for example through deploy/test/with-postgres.sh.
set -euo pipefail

test_name=TestFeedPositionsFollowCommitOrder
package=./src/server/internal/store
feed_file=src/server/internal/store/feed.go
marker='// lgtm:feed-lock'
assertion='writer B became visible before writer A'

fail() {
    printf '%s\n' "$*" >&2
    exit 1
}

command -v git >/dev/null 2>&1 || fail 'The feed lock mutation needs git.'
command -v go >/dev/null 2>&1 || fail 'The feed lock mutation needs go.'
[[ -n ${LGTM_TEST_POSTGRES:-} ]] ||
    fail 'The feed lock mutation needs LGTM_TEST_POSTGRES. Run it through deploy/test/with-postgres.sh.'

root=$(git rev-parse --show-toplevel 2>/dev/null) || fail 'The feed lock mutation must run inside the repository.'
[[ $root == "$(pwd -P)" ]] || fail 'The feed lock mutation must run from the repository root.'

changes=$(git status --porcelain --untracked-files=all -- src go.mod go.sum) ||
    fail 'The feed lock mutation could not read the checkout status.'
[[ -z $changes ]] ||
    fail 'The checkout has changes under src, go.mod or go.sum. The feed lock mutation tests only committed code.'

commit=$(git rev-parse --verify HEAD) || fail 'The feed lock mutation could not resolve HEAD.'
printf 'The feed lock mutation tests commit %s.\n' "$commit" >&2

copy=$(mktemp -d)
trap 'rm -rf -- "$copy"' EXIT
git archive --format=tar HEAD | tar -x -C "$copy" || fail 'The feed lock mutation could not archive HEAD.'
cd "$copy"

run_test() {
    go test -count=1 -v "$package" -run "^${test_name}\$" >"$copy/test.log" 2>&1
}

run_test || fail "The unmodified $test_name failed, so the mutation proves nothing."
grep -Eq "^--- PASS: ${test_name} " "$copy/test.log" ||
    fail "The unmodified run did not report a pass line for $test_name."

[[ $(grep -cF -- "$marker" "$feed_file") == 1 ]] ||
    fail "The feed lock marker must appear on exactly one line of $feed_file."
grep -vF -- "$marker" "$feed_file" >"$copy/feed.go.mutated"
mv -- "$copy/feed.go.mutated" "$feed_file"
go build ./... >"$copy/build.log" 2>&1 || fail 'The module did not build after the feed lock line was removed.'

if run_test; then
    fail "$test_name passed without the feed lock."
fi
grep -Eq "^--- FAIL: ${test_name} " "$copy/test.log" ||
    fail "The mutated run did not report a failure line for $test_name."
grep -Eq ": ${assertion}\$" "$copy/test.log" ||
    fail "The mutated run failed without the early visibility assertion."

printf '%s\n' "PASS C12 feed-lock mutation: $assertion"
