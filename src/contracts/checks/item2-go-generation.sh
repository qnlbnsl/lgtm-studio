#!/bin/bash
# Test of item 2 of the plan go-s0-contract: Go generation beside the TypeScript.
#
# It runs the commands of the plan's table "Tests written first" for item 2.
# The three probes (stale, untracked and guard) change files, so they run in a scratch
# copy of the repository, as the plan asks, and never in the checkout that holds this
# script. The copy runs `pnpm install --frozen-lockfile` first.
# Run it with: bash src/contracts/checks/item2-go-generation.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$ROOT" || exit 1

CONTRACT="pnpm --filter ./src/contracts"

[ -f src/contracts/gen/go/lgtm/v1/lgtmv1connect/clients.connect.go ] && pass "the generated Connect file for clients exists" || fail "the generated Connect file for clients exists"
for f in clients map preview session; do
  grep -q 'option go_package = "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1;lgtmv1";' "src/contracts/proto/lgtm/v1/$f.proto" \
    && pass "$f.proto has the go_package option" || fail "$f.proto has the go_package option"
done
grep -q 'protoc-gen-go' src/contracts/buf.gen.yaml && grep -q 'protoc-gen-connect-go' src/contracts/buf.gen.yaml && grep -q 'clean: true' src/contracts/buf.gen.yaml \
  && pass "buf.gen.yaml keeps clean and adds both Go plugins" || fail "buf.gen.yaml keeps clean and adds both Go plugins"
grep -q 'go test ./\.\.\.' src/contracts/package.json && grep -q -- '--untracked-files=all' src/contracts/package.json \
  && pass "the check script runs the Go tests and guards untracked files" || fail "the check script runs the Go tests and guards untracked files"

expect_status "the contract check passes" 0 $CONTRACT check
expect_status "go build ./... exits 0" 0 go build ./...
expect_status "go mod tidy -diff exits 0" 0 go mod tidy -diff

# Only the embedded descriptor lines of the old generated TypeScript may differ from master.
changed="$(git diff -U0 master 28208cb4 -- src/contracts/gen/lgtm 2>/dev/null | grep '^[-+][^-+]' | grep -vc 'fileDesc(')" || true
[ "$changed" = "0" ] && pass "only fileDesc lines changed in the old TypeScript" || fail "only fileDesc lines changed in the old TypeScript" "count: $changed"

expect_status "the website type check passes" 0 pnpm --filter ./src/frontend typecheck
expect_status "the website builds" 0 pnpm --filter ./src/frontend build
expect_status "the sample server type check passes" 0 pnpm --filter ./src/mock-server typecheck

# The probes, in a scratch copy.
COPY="$(scratch_copy)"
trap 'cd /; git -C "$COPY" status >/dev/null 2>&1 && rm -rf -- "$COPY"' EXIT
cd "$COPY" || exit 1
if ! pnpm install --frozen-lockfile >/dev/null 2>&1; then
  fail "pnpm install --frozen-lockfile in the scratch copy"
  cd "$ROOT"; finish; exit $?
fi
expect_status "probe: the clean copy passes the check first" 0 $CONTRACT check

# Stale probe.
sed -i '1s|^|// probe\n|' src/contracts/proto/lgtm/v1/session.proto
expect_status "stale probe: the check exits 1" 1 $CONTRACT check
git diff --name-only -- src/contracts/gen | grep -q 'session_pb.js' && pass "stale probe: the tracked diff lists session_pb.js" || fail "stale probe: the tracked diff lists session_pb.js"
git checkout -q -- . && git clean -fdq -- src/contracts/gen

# Untracked probe.
cat > src/contracts/proto/lgtm/v1/probe.proto <<'PROBE'
syntax = "proto3";
package lgtm.v1;
option go_package = "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1;lgtmv1";
message Probe {}
PROBE
(cd src/contracts && pnpm exec sh -c 'buf lint && buf generate') >/dev/null 2>&1 && pass "untracked probe: lint and generate exit 0" || fail "untracked probe: lint and generate exit 0"
new="$(git status --porcelain --untracked-files=all -- src/contracts/gen)"
for f in probe.pb.go probe_pb.js probe_pb.d.ts; do
  case "$new" in *"$f"*) pass "untracked probe: $f is listed as new";; *) fail "untracked probe: $f is listed as new" "$new";; esac
done
expect_status "untracked probe: the check exits 1" 1 $CONTRACT check
expect_status "guard probe: the check without its last step exits 0" 0 $CONTRACT exec sh -c 'buf lint && buf generate && go test ./... && git diff --exit-code -- gen'
cd "$ROOT"
finish
