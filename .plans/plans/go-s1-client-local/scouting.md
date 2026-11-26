# Scouting — go-s1-client-local

The planner of the Client team wrote this file on 2026-11-26, in phase 3 of the build loop.
It records what the planner read and ran, with file and symbol anchors.
No Go code exists in the repository yet, so most anchors are target paths with the nearest existing pattern beside them.

## What was read

| Source | What it gave this plan |
|---|---|
| `AGENTS.md` | The three parts, the team folders, and the writing rules. New code is Go. |
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md` | The source of truth. The sections this plan builds on are listed below. |
| `.plans/epics/go-rewrite/epic.md` | Sprint 1 for the Client team. The team edits only `src/client/`. Agreement point 4 is the layout of the state folder. |
| `.plans/plans/go-s1-client-local/plan.md` and `decisions.md` | The brief, with six work items and four points to settle. |
| `.plans/plans/go-s3-client-link/plan.md` | The later plan that reads the state folder, the outbox and the receipts. |
| `.plans/plans/go-s0-contract/plan.md` | The Go module, and the event and command envelopes of the link contract. |
| `docs/specs/2026-11-05-build-loop-design.md`, section "4 Build" | Tests are written first, fail on their assertion, and are then frozen. Risky tests must go red when a reviewer breaks the code. |
| `docs/issues.md`, issues 1632 to 1635 | Issue 1634 asks for test entry points outside `internal`. Issue 1635 sets sizes to low, medium and high. |
| The planning skill `world-class-plan`, with `references/templates.md` and `references/self-check.md` | The template and the probes of the 10 gates. |
| The model sheet of the harness-session skill, lines 86 to 115 | The work areas and the complexity levels. |
| `floorplan/plan.json`, key `files` | The planned files of the client, listed below. |
| `src/contracts/proto/lgtm/v1/clients.proto` | What the website shows about a machine today. |
| `packages/engine/src/harnesses/recipes.ts`, `packages/engine/src/harnesses/programs.ts`, `packages/engine/src/runner/harnesses.ts` | How the old engine found the harnesses and read their versions. |
| The Go standard library of the installed toolchain | Facts about `syscall`, `os/exec` and file syncing, listed below. |

## The statements of the design this plan builds on

The numbering counts the rows and lines of the design file as it is on `master`.

| Number | Where in the design | What it says, in short |
|---|---|---|
| D2 | Decisions table, row 2 | The server and the client are written in Go. |
| D7 | Decisions table, row 7 | A machine proves itself with a key pair. |
| D8 | Decisions table, row 8 | Version one drives every harness through the Agent Client Protocol (ACP) only. |
| D16 | Decisions table, row 16 | A client retries at least every 15 seconds and starts no web server. |
| D19 | Decisions table, row 19 | One user on a machine is a convenience for now. |
| R1 | Rules, line 1 | A part owns one folder under `internal`. |
| R6 | Rules, line 6 | Every command to a machine goes through the link's stored commands. |
| R9 | Rules, line 9 | Text from an agent is data. Secrets never reach a log, the board or the website. |
| R10 | Rules, line 10 | Tests run against a real PostgreSQL. Tests of the link use a real client against a real server. |
| C7 | Checks, line 7 | After approval the machine shows its facts, its load and its harnesses. |
| C10 | Checks, line 10 | A machine whose key the server does not know is refused. |
| C11 | Checks, line 11 | Events buffered while the server was down arrive once, in order. |
| Section "Joining", step 3 | Lines 249 | The private key stays in a file only its user can read, and it never leaves the machine. |
| Section "Commands go down" | Lines 283 to 286 | A command that arrives twice runs once. A command past its expiry is not run. |
| Section "Events come up" | Lines 290 to 292 | Events are numbered and kept on disk within a limit until acknowledged. |
| Section "What travels in wave 1" | Lines 302 to 308 | Facts, harnesses found and heartbeat travel up. |
| Section "The client in wave 1" | Lines 329 to 339 | Two commands, the state folder `~/.local/state/lgtm-client/`, and the four harnesses. |

## The blueprint's files for this plan

`floorplan/plan.json` names these client files that this plan touches:

| Blueprint path | Blueprint name | What this plan does with it |
|---|---|---|
| `src/client/cmd/lgtm-client/main.go` | Client start | Item 1 creates it with three commands. Plan `go-s3-client-link` fills in `join` and `run`. |
| `src/client/internal/config/config.go` | Machine settings | Item 1 creates it. It reads the settings file and finds the state folder. |
| `src/client/internal/enroll/credential.go` | Machine credential | Item 3 creates it. |
| `src/client/internal/sysinfo/sysinfo.go` | Machine facts and load | Item 4 creates it, with one file for each operating system. |
| `src/client/internal/harness/registry.go` | Harness definitions | Item 5 creates it with the names and programs only. Wave 2 adds how each harness starts. |
| `src/client/internal/harness/scan.go` | Harness scan | Item 5 creates it. |
| `src/client/internal/link/outbox.go` | Events waiting to be sent | Items 6 and 7 create it and `eventfile.go` beside it. |
| `src/client/internal/link/commands.go` | Command handling | Not this plan. Receipts get their own file, `receipts.go`, in item 8. |

The blueprint has no folder for the state folder itself.
This plan adds `src/client/internal/state/`, so that the key, the outbox and the receipts share one way of writing a file safely.

## The old engine as a pattern

The old engine is a reference only. Nothing is ported line by line.

- `packages/engine/src/harnesses/recipes.ts`, constant `installRecipes`: the program names are `claude`, `codex`, `gemini`, and `agent` for Cursor.
- `packages/engine/src/harnesses/programs.ts`, function `makeHarnessPrograms`, inner function `inspect`: it searches only absolute folders of the search path, runs `--version` with a time limit of 2,000 milliseconds, and matches `\b\d+\.\d+\.\d+\b` in the output. For Cursor it accepts output that names Cursor or starts with a date such as `2026.11.20`.
- `packages/engine/src/harnesses/programs.ts`, comment above the environment of child programs: the child never inherits `PATH`, proxies or provider secrets from the engine.
- `packages/engine/src/runner/harnesses.ts`, constant `definitions`: the ACP programs differ from the plain programs (`claude-agent-acp`, `codex-acp`, `gemini --acp`, `agent acp`). Wave 2 starts those. Wave 1 reports the plain programs and their versions, as the design's table "What travels in wave 1" says.
- `packages/engine/src/harnesses/programs.ts`, function `recordManagedInstallation`: the old engine wrote its own records with mode `0o600`. This plan keeps that mode.

## The website's view of a machine

`src/contracts/proto/lgtm/v1/clients.proto`, message `Client`:

- `os` is a string such as `Linux`.
- `cores` and `memory_gigabytes` are whole numbers.
- `cpu_percent` and `memory_percent` are whole percentages.
- `harnesses` is a list of `InstalledHarness`, each with `harness` and an optional `version`. The version is absent when the harness is not installed.

The types of item 4 and item 5 follow these fields, so that plan `go-s3-client-link` copies them without a conversion of meaning.

## Facts of the Go standard library

These were read in the source of the installed Go toolchain on 2026-11-26.

- `go version` on the development machine prints `go1.23.3 linux/amd64`. The design needs Go 1.26 or newer. The Contract team's `go.mod` decides how the newer toolchain is fetched.
- `syscall.Sysctl` and `syscall.SysctlUint32` exist for macOS in `src/syscall/syscall_bsd.go`. `syscall.Sysctl` drops one trailing zero byte from the value it returns. A decoder of a binary value must therefore pad the value back to its full length.
- `syscall.Flock` exists for Linux and for macOS (`src/syscall/zsyscall_linux_amd64.go`, `src/syscall/zsyscall_darwin_arm64.go`).
- `os.File.Sync` on macOS uses `F_FULLFSYNC`, and it falls back to `fsync` when the file system answers `ENOTSUP` (`src/internal/poll/fd_fsync_darwin.go`, method `FD.Fsync`).
- `os/exec.Cmd.WaitDelay` exists (`src/os/exec/exec.go`, `ErrWaitDelay`). It bounds the wait for output pipes that a grandchild holds open.
- `crypto/ed25519`, `crypto/x509.MarshalPKCS8PrivateKey`, `encoding/pem`, `hash/crc32` with the Castagnoli table, and `encoding/json` cover the rest. This plan needs no module outside the standard library.

## What was run

| Command | Result |
|---|---|
| `ls go.mod src/client src/server` | None exists. The rewrite has no Go code yet. |
| `go version` | `go1.23.3 linux/amd64`. |
| `grep` over `scripts/check-private.mjs` | The script forbids home folder paths of real users, private network addresses and quotations of the owner. A path that starts with `~/` passes. |

## Target paths and their nearest pattern

| Target path | Nearest existing pattern |
|---|---|
| `src/client/cmd/lgtm-client/main.go` | `packages/cli/src/bin.ts`, the old command-line entry. |
| `src/client/internal/config/config.go` | The settings section of the design, and `floorplan/plan.json` entry "Machine settings". |
| `src/client/internal/state/folder.go`, `src/client/internal/state/atomic.go` | `packages/engine/src/harnesses/programs.ts`, function `recordManagedInstallation`, which writes records with mode `0o600`. |
| `src/client/internal/enroll/credential.go` | The design's section "Joining", step 3. |
| `src/client/internal/sysinfo/*.go` | The fields of message `Client` in `src/contracts/proto/lgtm/v1/clients.proto`. |
| `src/client/internal/harness/registry.go`, `scan.go` | `packages/engine/src/harnesses/programs.ts`, function `makeHarnessPrograms`, inner function `inspect`. |
| `src/client/internal/link/eventfile.go`, `outbox.go`, `receipts.go` | The design's sections "Commands go down" and "Events come up". |

## Facts checked for the fold of review round 1

These were checked on 2026-11-26, at commit `3ef69e50`, while the findings of round 1 were folded.

| Check | Result | Used for |
|---|---|---|
| `ls -la .plans/scopes/client/backend/scope.json` | The file exists, with 537 bytes. | architect-r1 11. The sizing fold now says the scope file exists. |
| A fake script that runs `exec sleep 30` with an empty `PATH` | It fails with `exec: sleep: not found` and exits with 127 at once. A test of the time limit built that way passes without testing the limit. | architect-r1 9, critic-r1 8, logic-r1 9. The fakes now name `sleep` by an absolute path. |
| `command -v sleep` on the development machine | `/usr/bin/sleep`. The tests find the path with `exec.LookPath` and do not hard-code it. | The same findings. |
| `.plans/plans/go-s0-contract/draft/commands.proto`, enum `CommandOutcome` | It has `UNSPECIFIED`, `DONE`, `FAILED`, `EXPIRED` and `UNSUPPORTED`. It has no `INTERRUPTED`. | logic2-r1 3. The result file asks the Contract team for the outcome. |
| `.plans/plans/go-s0-contract/draft/link.proto`, message `Welcome`, field `last_event_number = 3` | The server tells the client one number, with no name of the outbox it belongs to. | architect-r1 3 and the others of that group. The result file asks for an epoch beside it. |
| `.plans/plans/go-s0-contract/draft/events.proto`, message `EventAck`, field `through = 1` | The acknowledgement is cumulative. | Decision 40. |
| `deploy/test/checks.sh`, function `machines` | It builds `./src/client/cmd/lgtm-client` with `go build -o`, and the user-flow test starts the client with `XDG_STATE_HOME` set. | critic-r1 12, the pattern of task 1.2. The rule for the state folder already reads `XDG_STATE_HOME`. |
| `packages/engine/src/workflows/store.ts`, class `WorkflowStore` | Its save opens a temporary file with `O_EXCL` and `O_NOFOLLOW`, syncs it, and links it for a new record or renames it over an old one. | critic-r1 12, the pattern of task 2.2. |
| `go build -o <file> ./src/client/...` with more than one package | Go refuses to write several packages to one file, so the old cross build test could never pass. | architect-r1 10. |

## New target paths of round 1 and their nearest pattern

| Target path | Nearest existing pattern |
|---|---|
| `src/client/internal/state/testhook.go`, `SetSyncFolderForTest` | The unexported variable `syncFolder` in `src/client/internal/state/atomic.go` of the same item. |
| `src/client/internal/harness/trust.go`, `trustedProgram` | `Open` in `src/client/internal/state/folder.go`, which applies the same rule to folders. |
| `src/client/internal/sysinfo/sysinfo_darwin_test.go`, `TestReadLoadOnMacMatchesSystem` | `src/client/internal/sysinfo/sysinfo_linux_test.go`, `TestReadLoadOnLinux`. |
| `src/client/internal/sysinfo/testdata/meminfo`, `testdata/loadavg` | The Linux files `/proc/meminfo` and `/proc/loadavg`. |
