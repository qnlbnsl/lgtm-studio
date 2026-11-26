#!/bin/bash
# Test of item 1 of the plan go-s0-contract: the Go module at the repository root.
#
# It runs the seven commands of the plan's table "Tests written first" for item 1,
# at the repository root, and compares each with the expected output.
# Run it with: bash src/contracts/checks/item1-go-module.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$ROOT" || exit 1

expect_output "go list -m names the module" "github.com/qnlbnsl/lgtm-studio" go list -m
expect_prefix "go version is Go 1.26" "go version go1.26." go version
expect_output "protoc-gen-go is pinned to v1.36.12" "protoc-gen-go v1.36.12" go tool protoc-gen-go --version
expect_output "protoc-gen-connect-go is pinned to 1.21.1" "1.21.1" go tool protoc-gen-connect-go --version
expect_output "go mod verify passes" "all modules verified" go mod verify
expect_status "go build ./... exits 0" 0 go build ./...
expect_status "go mod tidy -diff exits 0" 0 go mod tidy -diff

# The content rules of go.mod that the plan fixes.
if [ -f go.mod ]; then
  grep -qx 'go 1.26.0' go.mod && pass "go.mod says go 1.26.0" || fail "go.mod says go 1.26.0"
  grep -q 'connectrpc.com/connect v1.21.1' go.mod && pass "go.mod requires connect v1.21.1" || fail "go.mod requires connect v1.21.1"
  grep -q 'google.golang.org/protobuf v1.36.12' go.mod && pass "go.mod requires protobuf v1.36.12" || fail "go.mod requires protobuf v1.36.12"
  grep -q 'connectrpc.com/connect/cmd/protoc-gen-connect-go' go.mod && pass "go.mod has the connect tool" || fail "go.mod has the connect tool"
  grep -q 'google.golang.org/protobuf/cmd/protoc-gen-go' go.mod && pass "go.mod has the go tool" || fail "go.mod has the go tool"
else
  fail "go.mod exists"
fi
[ -f go.sum ] && pass "go.sum exists" || fail "go.sum exists"
finish
