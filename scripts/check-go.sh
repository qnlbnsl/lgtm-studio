#!/usr/bin/env bash
# Checks Go formatting, module dependencies, vet diagnostics, and tests.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

unformatted=$(
  git ls-files --cached --others --exclude-standard -z -- '*.go' |
    xargs -0 gofmt -l --
)
if [[ -n "$unformatted" ]]; then
  printf 'Go files need formatting.\n%s\n' "$unformatted" >&2
  exit 1
fi

go mod tidy -diff
go vet ./...
go test ./...
