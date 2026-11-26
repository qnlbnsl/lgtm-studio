#!/bin/bash
# Shared helpers of the shell tests of sprint 0 (items 1, 2 and 7 of the plan go-s0-contract).
#
# A test script sources this file, calls `expect_*` for each command of the plan,
# and ends with `finish`. A failed expectation prints FAIL and the script goes on,
# so one run shows every failure. `finish` exits 1 when anything failed.
#
# The scripts need Node and Go on the search path. A plain shell has neither Node
# nor a pnpm that works, so the helper adds the Node of this machine when it exists.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
for d in "$HOME"/.nvm/versions/node/*/bin; do
  [ -d "$d" ] && PATH="$d:$PATH"
done
export PATH
export GOFLAGS="${GOFLAGS:--mod=mod}"
unset GOFLAGS
FAILS=0
PASSES=0

pass() { PASSES=$((PASSES + 1)); echo "PASS: $1"; }
fail() { FAILS=$((FAILS + 1)); echo "FAIL: $1"; [ -n "${2:-}" ] && printf '%s\n' "$2" | sed 's/^/    /'; return 0; }

# expect_output <name> <expected> <command...>: the command's output, with the trailing newline removed, equals <expected>.
expect_output() {
  local name="$1" want="$2"; shift 2
  local got
  got="$("$@" 2>&1)" || true
  if [ "$got" = "$want" ]; then pass "$name"; else fail "$name" "wanted: $want
got:    $got"; fi
}

# expect_prefix <name> <prefix> <command...>: the first line of the output starts with <prefix>.
expect_prefix() {
  local name="$1" want="$2"; shift 2
  local got
  got="$("$@" 2>&1 | head -n 1)" || true
  case "$got" in "$want"*) pass "$name";; *) fail "$name" "wanted a line that starts with: $want
got: $got";; esac
}

# expect_status <name> <status> <command...>: the command exits with <status>.
expect_status() {
  local name="$1" want="$2"; shift 2
  local out rc=0
  out="$("$@" 2>&1)" || rc=$?
  if [ "$rc" -eq "$want" ]; then pass "$name"; else fail "$name" "wanted exit status $want, got $rc
$(printf '%s\n' "$out" | tail -n 8)"; fi
}

# scratch_copy: copies the files of the repository (tracked and untracked, not ignored)
# into a new folder from mktemp and makes it a git repository with one commit, so the
# probes never touch the builder's checkout. It prints the folder.
scratch_copy() {
  local dir
  dir="$(mktemp -d)"
  (cd "$ROOT" && git ls-files --cached --others --exclude-standard -z | while IFS= read -r -d '' f; do
     [ -e "$f" ] && printf '%s\0' "$f"
   done | tar --null -T - -cf -) | tar -xf - -C "$dir"
  (cd "$dir" && git init -q . && git add -A && git -c user.name=probe -c user.email=probe@localhost commit -q -m probe)
  echo "$dir"
}

finish() {
  echo "$PASSES passed, $FAILS failed"
  [ "$FAILS" -eq 0 ]
}
