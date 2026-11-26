#!/usr/bin/env bash
# Checks Go formatting, module dependencies, vet diagnostics, tests, and frozen link history.
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

link_record=src/contracts/testdata/link-frozen.json
link_base=${LGTM_LINK_BASE:-}
if [[ -z "$link_base" ]]; then
  # Search every parent of HEAD so a merge cannot hide the freeze on another parent.
  prior_record=$(git rev-list --full-history -1 'HEAD^@' -- "$link_record")
  shallow=$(git rev-parse --is-shallow-repository)
  if [[ -n "$prior_record" || "$shallow" == true ]]; then
    printf 'The link is frozen, so set LGTM_LINK_BASE to the commit of the freeze.\n' >&2
    exit 1
  fi
else
  # Compare each recorded revision at its original line number.
  if ! link_commit=$(git rev-parse --verify --end-of-options "${link_base}^{commit}") ||
    ! git show "$link_commit:$link_record" | awk '
      FILENAME == "-" {
        if (index($0, "\"revision\"")) {
          revision[FNR] = $0
          remaining++
        }
        next
      }
      (FNR in revision) && $0 == revision[FNR] {
        matched++
        remaining--
      }
      END { if (!matched || remaining) exit 1 }
    ' - "$link_record"; then
    printf 'The frozen link record must keep every revision line from LGTM_LINK_BASE in place.\n' >&2
    exit 1
  fi
fi
