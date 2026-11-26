---
topic: go-s1-client-local
status: attack
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# What the client does on its own machine

- **Sprint:** 1. Each part stands alone
- **Team:** Client
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)
- **Brief:** written by the orchestrator on 2026-11-25. The planner of the Client team turned it into this plan on 2026-11-26.

## Overview

The client is the program that runs on each machine.
In sprint 1 it does not talk to a server yet.
This plan builds everything the client does on its own machine, so that plan `go-s3-client-link` in sprint 3 only has to connect the pieces to the server.

When this plan is done, the program `lgtm-client` exists and builds for Linux and for macOS.
It knows where its state folder is, and it refuses to use a folder that other users can read.
It can make the key pair of the machine and keep the private key in a file that only its user can read.
It can read the facts of the machine, such as the operating system, the cores, the memory and the current load.
It can find out which of the four coding-agent programs of the design are installed, and which version each one has.
It keeps two records on disk: the events that wait to be sent to the server, and a receipt of each command it ran.

The two records carry the promises of the design that no event is lost and that no command runs twice.
Both must survive a crash in the middle of a write.
The plan handles this with one way of writing a file, used everywhere: write a temporary file, sync it to disk, rename it over the final name, and sync the folder.
A crash then leaves either the old file or the new one, never a mix.
Tests prove it by killing a child process at random moments and at each step of a write, and by checking the order of the syncs.

The plan also fixes the layout of the state folder.
Plan `go-s3-client-link` reads the same files later, so every name, format and permission is written down here.

No owner decision is needed to start.
Several choices where the design is silent are logged as assumptions, and the owner can change them later without rework: the folder for repositories, the agent limit, the size of the event buffer, how long receipts are kept, and the formula for the load on macOS.

## Key decisions

**A full event buffer refuses new events.**
The design says that no event is lost.
Dropping the oldest event would break that and leave a gap in the numbers.
The program that makes an event therefore gets an error and decides what to do.

**Every event is on disk before it gets a number that can be sent.**
If an event could be sent before it was safe on disk, a crash could give its number to a different event.
The server would then drop that event as a repeat.
This costs two disk syncs for each event, which is fine for the few events of wave 1.
Wave 2 can change the storage behind the same interface if agents produce many more events.

**A command runs at most once, even across a crash.**
The client writes a receipt that says "started" before it runs a command.
If the client stops in the middle, that receipt becomes "interrupted" when the client starts again, and the command is not run again.
The server can send a new command if it still wants the work done.

**The harness scan never runs a shell and never inherits secrets.**
It runs each program directly with the one argument `--version`, with only three variables in its environment, and with a time limit of 3 seconds.
It keeps only the version text, so nothing else a program prints can reach the website.
Tests use fake programs in a temporary folder, so they never touch the real programs on the machine.

**The load on macOS uses the load average.**
macOS gives no busy time of the processor to a Go program without a C library call.
The client therefore reports the load of the last minute divided by the number of cores, on Linux and on macOS alike.
This should be checked on a real Mac before sprint 3 ends.

**The plan needs nothing outside Go's standard library.**
The module file `go.mod` belongs to the Contract team, so a new library would mean a request and a wait.

Full log → `decisions.md`

## Workstreams

### The client program starts and keeps its state folder private

The program `lgtm-client` gets its three commands for sprint 1: `version` works, and `join` and `run` say that they are not built yet.
It finds its state folder, creates it so that only its user can open it, and refuses a folder that others can read.
Only one client may use a state folder at a time, because two would hand out the same event numbers.
This workstream also builds the one safe way of writing a file that the rest of the plan uses.
The main risk is a write that a crash cuts in half, and tests stop a child process at each step of a write to prove that it cannot happen.
When it is done, `go test ./src/client/...` passes, and a test builds the program for Linux and macOS.

### The machine has a key that never leaves it

The client makes an Ed25519 key pair, the kind of key the design names, and keeps the private key in `machine.key` with the mode that lets only its user read it.
It refuses to load a key file that other users can read, that is a link to another file, or that another user owns.
It never overwrites an existing key, because that would erase the machine's identity.
The risk is a key that leaks into a log or a file that others can read, and tests try each of these.
When it is done, the key tests pass, and a reviewer who removes the permission check sees them fail.

### The client can describe its machine and its harnesses

The client reads the facts that the Machines page of the website shows: the host name, the operating system, the cores, the memory, its own version, and the load of the processor and of the memory.
It also looks for the four harnesses of the design and reads the version of each one that is installed.
The main risks are a program that never answers, and a program that prints a secret.
A time limit handles the first, and keeping only the version text handles the second.
When it is done, the tests pass with fake programs, and no real harness on the machine is run.

### Nothing the client must send or has done is lost in a crash

The client keeps the events that wait for the server in an outbox on disk, numbered from 1, until the server acknowledges them.
It keeps a receipt of each command, so that a command that arrives twice runs once and the first result is sent again.
A command that arrives after its expiry is not run.
The main risk is a crash in the middle of a write, which could lose an event, reuse its number, or run a command twice.
Tests kill a child process again and again while it writes, and they check after each kill that every event that got a number is whole and that no command can run again.
When it is done, those tests pass, and a reviewer who moves the write after the run sees them fail.

## Rollout order

1. Wait for `go.mod` from plan `go-s0-contract`, because no Go code builds without it.
2. Build items 1, 2, 4 and 5 at the same time. They share no file and need nothing from each other.
3. Build items 3, 6 and 8 when item 2 is merged, because all three write files through it.
4. Build item 7 when item 6 is merged, because the queue reads the event files of item 6.
5. Run the whole suite of the client and the build for macOS on the merged work, in phase 5 of the build loop.
6. Hand the layout of the state folder to plan `go-s3-client-link` before its planner freezes it.

Nothing in this plan is hard to reverse.
No step needs the owner.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**The rules that every item of this plan keeps**

- The team edits only `src/client/`. Every file named below is under it.
- The module path is `github.com/qnlbnsl/lgtm-studio`, so the import path of a package is `github.com/qnlbnsl/lgtm-studio/src/client/internal/<name>`.
- Only the standard library of Go is imported.
- No test needs a server, a database or the network. `go test ./src/client/...` passes with the network turned off.
- Every test also passes under `go test -race`.
- The client builds for Linux and macOS with `CGO_ENABLED=0`. A file that only one system can compile carries a build line, `//go:build linux` or `//go:build darwin`.
- A test that needs a crash runs the test binary again as a child process. The child is started with `os.Executable()`, the argument `-test.run=^TestHelperProcess$`, and the variable `LGTM_TEST_HELPER` naming what it does. `TestHelperProcess` returns at once when `LGTM_TEST_HELPER` is empty, so it never runs on its own.
- Every work item has the scope `client/backend` and the work area `backend`.

**The layout of the state folder**

Plan `go-s3-client-link` reads this layout. It is the agreement of point 4 in the epic's section "What the teams must agree on".

| Path inside the folder | Kind and mode | Format | Written by | Read by |
|---|---|---|---|---|
| (the folder) | folder, `0700` | | item 2, `state.Open` | every item |
| `lock` | file, `0600` | empty. An exclusive `flock` on it is held while a client runs. | item 2 | item 2 |
| `settings.json` | file, `0600` | JSON (JavaScript Object Notation): `{"version":1,"repositories_folder":"~/…","agent_limit":4}`. Both fields after `version` are optional. | the owner or the installer | item 1, `config.LoadSettings` |
| `machine.key` | file, `0600` | one PEM (Privacy-Enhanced Mail) block of type `PRIVATE KEY`, holding the Ed25519 key in PKCS #8 (Public-Key Cryptography Standards number 8) form | item 3, `enroll.Create` | item 3, `enroll.Load` |
| `server.json` | file, `0600` | JSON with `"version":1`. Its other fields belong to plan `go-s3-client-link`: the server's address, its public key, the fingerprint of its certificate, and the machine's identifier. | plan `go-s3-client-link`, through `state.WriteAtomic` | plan `go-s3-client-link` |
| `outbox/` | folder, `0700` | | item 2 | items 6 and 7 |
| `outbox/acked` | file, `0600` | the highest acknowledged event number in decimal, then one newline | item 6 | item 6 |
| `outbox/<number>.event` | file, `0600` | binary, described in item 6. The number has 20 digits with leading zeros, such as `00000000000000000042.event`. | item 7 | items 6 and 7 |
| `receipts/` | folder, `0700` | | item 2 | item 8 |
| `receipts/<command id>.json` | file, `0600` | JSON, described in item 8 | item 8 | item 8 |
| `.tmp-<random>` in any of these folders | file, `0600` | an unfinished write. Every opener removes it. | item 2 | nobody |

The folder is found by this rule, from decision 8:

1. `LGTM_CLIENT_STATE_DIR`, when it is set and not empty. It must be an absolute path.
2. Otherwise `$XDG_STATE_HOME/lgtm-client`, when `XDG_STATE_HOME` is an absolute path. XDG is the X Desktop Group, whose rule for folders this follows.
3. Otherwise `$HOME/.local/state/lgtm-client`. This is the folder the design names.

**The statements of the design, by number**

The numbering counts the design file as it is on `master`, as the epic says.
D2 is the Go rewrite. D7 is the key pair of a machine. D8 is the Agent Client Protocol (ACP) only. D16 is "a client starts no web server". D19 is one user on a machine.
R1 is one folder for each part. R6 is commands through the link's stored commands. R9 is "text from an agent is data, and secrets never reach a log". R10 is real tests.
C7 is "the machine shows its facts, its load and its harnesses". C10 is "an unknown key is refused". C11 is "buffered events arrive once, in order".
scouting.md lists every statement with its line.

**WS-1: The client program starts and keeps its state folder private**

This workstream gives the client a program to run and a safe place for its state.
Item 1 is the program and its settings.
Item 2 is the state folder and the one safe way of writing a file.
Every later item writes through item 2, which is why item 2 carries the crash tests of the write itself.

**Item 1. The program and its settings**

- **Files:** `src/client/cmd/lgtm-client/main.go`, `src/client/cmd/lgtm-client/main_test.go`, `src/client/cmd/lgtm-client/build_test.go`, `src/client/internal/config/config.go`, `src/client/internal/config/config_test.go`.
- **Needs first:** `go.mod` from plan `go-s0-contract`. Runs at the same time as items 2, 4 and 5.
- **Work area:** `backend`. **Complexity:** low, because it is a fixed set of commands and a small file reader with no design choice left.
- **Scope:** `client/backend`.
- **Design statements:** D2, D16 (the program has no command that serves the web), R1, and the section "The client in wave 1".
- **Risky:** no.
- **Public names that others build against:**
  - In `main`: `func run(args []string, stdout, stderr io.Writer) int`, and `var version = "0.0.0-dev"`, which an installer sets with `-ldflags "-X main.version=<v>"`.
  - In `config`: `const StateDirVariable = "LGTM_CLIENT_STATE_DIR"`; `func StateDir(getenv func(string) string) (string, error)`; `type Settings struct { RepositoriesFolder string; AgentLimit uint32 }`; `func DefaultSettings(home string) Settings`; `func LoadSettings(path, home string) (Settings, error)`; `var ErrNoStateDir`, `var ErrSettingsInvalid`.
- **Behaviour:**
  - `run` with `version` writes the version and a newline to `stdout` and returns 0.
  - `run` with `join` or `run` writes one sentence to `stderr` that names the command and says it is not built yet, and returns 1.
  - `run` with no argument or an unknown one writes the usage to `stderr` and returns 2.
  - `main` calls `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))`.
  - `StateDir` follows the three steps of the layout above. A relative `LGTM_CLIENT_STATE_DIR` is an error. A relative `XDG_STATE_HOME` is skipped. No usable value returns `ErrNoStateDir`.
  - `DefaultSettings(home)` returns the folder `<home>/.local/share/lgtm-client/repositories` and the agent limit 4.
  - `LoadSettings` returns the defaults when the file does not exist. It refuses with `ErrSettingsInvalid` a file that is not JSON, a `version` other than 1, an unknown field, a folder that is neither absolute nor starts with `~/`, and an agent limit outside 1 to 64. A folder that starts with `~/` is expanded against `home`.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `main_test.go` | `TestVersionPrintsVersion` | `run([]string{"version"}, …)` returns 0, and `stdout` is the value of `version` and a newline. |
| `main_test.go` | `TestJoinAndRunAreNotBuiltYet` | For `join` and for `run`, the return is 1, `stdout` is empty, and `stderr` holds the command's name and the words `not built yet`. |
| `main_test.go` | `TestUnknownCommandPrintsUsage` | For no argument and for `serve`, the return is 2, and `stderr` names the three commands `join`, `run` and `version`. |
| `build_test.go` | `TestBuildsForLinuxAndMacOS` | Skipped under `-short`. For each pair of `linux` or `darwin` with `amd64` or `arm64`, it runs `go build -o <temp> ./src/client/...` from the module root with `CGO_ENABLED=0`, and each build exits 0. It fails, and does not skip, when `go` is not on the `PATH`. |
| `config_test.go` | `TestStateDirPrefersVariable` | With `LGTM_CLIENT_STATE_DIR=/a/b`, `XDG_STATE_HOME=/x` and `HOME=/h`, the result is `/a/b`. |
| `config_test.go` | `TestStateDirRefusesRelativeVariable` | With `LGTM_CLIENT_STATE_DIR=rel`, the error is not nil. |
| `config_test.go` | `TestStateDirUsesXDGStateHome` | With `XDG_STATE_HOME=/x` and `HOME=/h`, the result is `/x/lgtm-client`. |
| `config_test.go` | `TestStateDirSkipsRelativeXDG` | With `XDG_STATE_HOME=rel` and `HOME=/h`, the result is `/h/.local/state/lgtm-client`. |
| `config_test.go` | `TestStateDirWithoutHome` | With no variable set, the error is `ErrNoStateDir`. |
| `config_test.go` | `TestLoadSettingsMissingFileGivesDefaults` | A path that does not exist gives `DefaultSettings(home)` and no error. |
| `config_test.go` | `TestLoadSettingsReadsFields` | `{"version":1,"repositories_folder":"~/repos","agent_limit":8}` gives `<home>/repos` and 8. |
| `config_test.go` | `TestLoadSettingsRefusesBadFiles` | Each of these gives `ErrSettingsInvalid`: text that is not JSON, `"version":2`, an unknown field `"agentLimit"`, the folder `repos`, the agent limit 0, and the agent limit 65. |

- **Acceptance:** `go test ./src/client/cmd/... ./src/client/internal/config/...` passes. `go run ./src/client/cmd/lgtm-client version` prints `0.0.0-dev`.

**Item 2. The state folder and safe writes**

- **Files:** `src/client/internal/state/folder.go`, `src/client/internal/state/atomic.go`, `src/client/internal/state/testhook.go`, `src/client/internal/state/folder_test.go`, `src/client/internal/state/atomic_test.go`, `src/client/internal/state/crash_test.go`.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 4 and 5.
- **Work area:** `backend`. **Complexity:** medium, because the method of writing is known and fits in two small files, and the risk is covered by named crash tests.
- **Scope:** `client/backend`.
- **Design statements:** D19 (the state belongs to one user), R9 (no other user reads the state), the section "Joining" step 3 (a file only its user can read), and the sections "Commands go down" and "Events come up", whose records stand on these writes.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Writing the final file in place instead of through a temporary file makes `TestWriteAtomicCrashAtEachStep` fail.
  2. Removing the sync of the temporary file makes `TestWriteAtomicSyncsBeforeRename` fail.
  3. Removing the check of group and other bits in `Open` makes `TestOpenRefusesExposedFolder` fail.
  4. Removing the `flock` makes `TestOpenRefusesSecondHolder` fail.
  5. Using a rename instead of a link in `CreateExclusive` makes `TestCreateExclusiveNeverReplaces` fail.
- **Public names that others build against:**
  - Constants `LockFile = "lock"`, `SettingsFile = "settings.json"`, `KeyFile = "machine.key"`, `ServerFile = "server.json"`, `OutboxDir = "outbox"`, `ReceiptsDir = "receipts"`, `TempPrefix = ".tmp-"`.
  - `type Paths struct { Dir, Lock, Settings, Key, Server, Outbox, Receipts string }` and `func PathsIn(dir string) Paths`.
  - `type Folder struct { Paths Paths }` with `func Open(dir string) (*Folder, error)` and `func (f *Folder) Close() error`.
  - `func WriteAtomic(dir, name string, data []byte) error`, which writes mode `0600`.
  - `func CreateExclusive(dir, name string, data []byte) error`, which writes mode `0600` and returns an error that wraps `os.ErrExist` when the name exists.
  - `func RemoveTemps(dir string) error` and `func SyncDir(dir string) error`.
  - `var ErrNotAFolder`, `ErrFolderExposed`, `ErrFolderNotOwned`, `ErrFolderLocked`, `ErrBadName`.
  - Unexported, for tests in the package only: `var crashHook func(step string)`, `var syncFile func(*os.File) error`, `var syncFolder func(dir string) error`. The steps are `"temp-created"`, `"temp-written"`, `"temp-synced"`, `"placed"` and `"dir-synced"`, in that order. `"placed"` is the rename, or the link for `CreateExclusive`.
  - In `testhook.go`: `func SetCrashHookForTest(hook func(step string)) (restore func())`, which sets `crashHook` and returns a function that puts the old one back. Its comment says that only tests call it. When nothing calls it, `crashHook` stays nil and the writes call no hook.
- **Behaviour:**
  - `Open` creates the folder and its missing parents with mode `0700`, and then creates `outbox/` and `receipts/` the same way.
  - `Open` refuses the folder and each subfolder when it is a symbolic link or not a folder (`ErrNotAFolder`), when its mode has any bit of group or others (`ErrFolderExposed`), and when its owner is not the user who runs the client (`ErrFolderNotOwned`). It never changes the mode of a folder that already exists.
  - `Open` creates `lock` with mode `0600` and takes `syscall.Flock` with `LOCK_EX|LOCK_NB`. When another holder has it, `Open` returns `ErrFolderLocked`. `Close` releases it.
  - `Open` calls `RemoveTemps` on the folder and both subfolders.
  - `WriteAtomic` and `CreateExclusive` refuse with `ErrBadName` a name that is empty, `.` or `..`, holds a path separator, or starts with `TempPrefix`.
  - `WriteAtomic` creates `.tmp-<16 random hexadecimal characters>` with `O_CREATE|O_EXCL` and mode `0600`, writes the data, syncs the file, closes it, renames it over the name, and syncs the folder. On any error it removes the temporary file.
  - `CreateExclusive` does the same, except that it links the temporary file to the name with `os.Link`, which fails when the name exists, and then removes the temporary file.
  - Each error message is one sentence that names the folder or the file.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `folder_test.go` | `TestOpenCreatesPrivateFolders` | `Open` on a path inside `t.TempDir()` that does not exist creates the folder, `outbox/` and `receipts/`, each with mode exactly `0700`, and `lock` with mode exactly `0600`. |
| `folder_test.go` | `TestPathsInNamesEveryFile` | `PathsIn("/s")` gives `/s/lock`, `/s/settings.json`, `/s/machine.key`, `/s/server.json`, `/s/outbox` and `/s/receipts`. |
| `folder_test.go` | `TestOpenRefusesExposedFolder` | For an existing folder of mode `0750`, then `0705`, `Open` returns `ErrFolderExposed`, and the mode is unchanged afterwards. The same holds for `outbox/` of mode `0755` inside a folder of mode `0700`. |
| `folder_test.go` | `TestOpenRefusesSymlink` | When the path is a symbolic link to a folder of mode `0700`, `Open` returns `ErrNotAFolder`. |
| `folder_test.go` | `TestOpenRefusesSecondHolder` | A second `Open` of the same folder, while the first is open, returns `ErrFolderLocked`. After the first `Close`, a new `Open` succeeds. The second `Open` also runs in a child process (helper `hold-lock`), so the test covers two processes and not only one. |
| `folder_test.go` | `TestOpenRemovesTemps` | A file `.tmp-abc` in the folder and one in `outbox/` are gone after `Open`. A file `keep.json` stays. |
| `atomic_test.go` | `TestWriteAtomicWritesPrivateFile` | After `WriteAtomic(dir, "a.json", data)`, the file holds exactly `data`, its mode is exactly `0600`, and no name with the prefix `.tmp-` is left. |
| `atomic_test.go` | `TestWriteAtomicReplaces` | Writing `A` and then `B` leaves exactly `B`. |
| `atomic_test.go` | `TestWriteAtomicRefusesBadNames` | `""`, `"."`, `".."`, `"a/b"` and `".tmp-x"` each return `ErrBadName`, and the folder is unchanged. |
| `atomic_test.go` | `TestWriteAtomicSyncsBeforeRename` | With `crashHook` set to record each step and `syncFile` and `syncFolder` replaced by recorders that note their call, the recorded order is `temp-created`, `temp-written`, the file sync, `temp-synced`, `placed`, the folder sync, `dir-synced`. |
| `atomic_test.go` | `TestCreateExclusiveNeverReplaces` | `CreateExclusive` of `k` with `A` succeeds. A second call with `B` returns an error for which `errors.Is(err, os.ErrExist)` is true, and the file still holds exactly `A`. No `.tmp-` name is left. |
| `crash_test.go` | `TestWriteAtomicCrashAtEachStep` | For each of the five steps, a child (helper `write-crash`) writes `x` with content `A`, then writes `x` with content `B` of 256 kibibytes, with `crashHook` set to `os.Exit(86)` at that step. The parent asserts that the child exited with 86 and that `x` holds exactly `A` or exactly `B`, never another content and never nothing. After `RemoveTemps`, no `.tmp-` name is left. For the steps `temp-created`, `temp-written` and `temp-synced`, the content is exactly `A`. |
| `crash_test.go` | `TestCreateExclusiveCrashAtEachStep` | For each step, a child (helper `create-crash`) creates `k` with content `A` and exits with 86 at that step. The parent asserts that `k` is either missing or holds exactly `A`. For `placed` and `dir-synced`, it holds exactly `A`. |

- **Acceptance:** `go test -race ./src/client/internal/state/...` passes. The break reviewer's report names the five mutations above and the test that went red for each.

**Tasks of WS-1:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the commands of the program | `src/client/cmd/lgtm-client/main.go`, `run` (target). Pattern: `packages/cli/src/bin.ts`. |
| 1.2 | Write the cross build test | `src/client/cmd/lgtm-client/build_test.go`, `TestBuildsForLinuxAndMacOS` (target). |
| 1.3 | Write the finder of the state folder and the settings reader | `src/client/internal/config/config.go`, `StateDir` and `LoadSettings` (target). Pattern: `floorplan/plan.json`, entry "Machine settings". |
| 2.1 | Write the checks and the lock of the folder | `src/client/internal/state/folder.go`, `Open` (target). Pattern: `packages/engine/src/harnesses/programs.ts`, `recordManagedInstallation`. |
| 2.2 | Write the safe writes | `src/client/internal/state/atomic.go`, `WriteAtomic` and `CreateExclusive` (target). |
| 2.3 | Write the exported test hook | `src/client/internal/state/testhook.go`, `SetCrashHookForTest` (target). |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The program answers its three commands. | `go test -run 'TestVersionPrintsVersion|TestJoinAndRunAreNotBuiltYet|TestUnknownCommandPrintsUsage' ./src/client/cmd/lgtm-client/` passes. |
| 2 | The program builds for Linux and macOS. | `go test -run TestBuildsForLinuxAndMacOS ./src/client/cmd/lgtm-client/` passes. |
| 3 | The state folder is created private and a folder others can read is refused. | `go test -run 'TestOpenCreatesPrivateFolders|TestOpenRefusesExposedFolder|TestOpenRefusesSymlink' ./src/client/internal/state/` passes. |
| 4 | Only one client uses a state folder. | `go test -run TestOpenRefusesSecondHolder ./src/client/internal/state/` passes. |
| 5 | A crash in a write leaves the old file or the new one. | `go test -run 'TestWriteAtomicCrashAtEachStep|TestCreateExclusiveCrashAtEachStep' ./src/client/internal/state/` passes. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A crash leaves a half-written file that a later reader trusts. | high | The temporary file and the rename of item 2, proven by `TestWriteAtomicCrashAtEachStep`. |
| A loss of power undoes a rename that was not synced. | medium | The folder is synced after every rename. `TestWriteAtomicSyncsBeforeRename` proves the order. A real loss of power is not tested, and the review lens checks the code. |
| A folder sync behaves differently on macOS. | medium | Go's `os.File.Sync` uses `F_FULLFSYNC` and falls back to `fsync` on `ENOTSUP`. The tests run on Linux, so phase 5 runs `go test ./src/client/...` on a Mac when one is available, and the gap is logged when none is. |
| The test that builds for four targets is slow. | low | It is skipped under `-short`, and the full suite runs it. |

**Verify:** the tests of items 1 and 2, and `go test -race ./src/client/...`. Device check: none for Linux. A run of `go test ./src/client/internal/state/...` on a Mac is wanted in phase 5.

**Rollout detail:** steps 1 and 2 of the rollout order.

**WS-2: The machine has a key that never leaves it**

This workstream makes the key that later proves the machine to the server, by D7.
Plan `go-s3-client-link` signs the server's challenge with it and makes the machine's certificate from it.
The key is the most sensitive file on the machine, so this item refuses every way it could be read by someone else.

**Item 3. The key of the machine**

- **Files:** `src/client/internal/enroll/credential.go`, `src/client/internal/enroll/credential_test.go`.
- **Needs first:** item 2, for `state.CreateExclusive`. Runs at the same time as items 6 and 8.
- **Work area:** `backend`. **Complexity:** medium, because the standard library does the cryptography, and the work is careful file handling with a clear test for each refusal.
- **Scope:** `client/backend`.
- **Design statements:** D7, the section "Joining" step 3, R9, and C10, whose refusal of an unknown key stands on this key.
- **Risky:** yes, for keys. These mutations must each make a named test fail:
  1. Removing the check of group and other bits in `Load` makes `TestLoadRefusesExposedKey` fail.
  2. Writing the key with mode `0644` makes `TestCreateWritesPrivateKeyFile` fail.
  3. Opening the file without `O_NOFOLLOW` makes `TestLoadRefusesSymlink` fail.
  4. Removing the methods `String` and `GoString` makes `TestCredentialNeverPrintsKey` fail.
  5. Replacing `state.CreateExclusive` with `state.WriteAtomic` makes `TestCreateRefusesExistingKey` fail.
- **Public names that others build against:**
  - `type Credential struct` with no exported field.
  - `func Create(path string) (*Credential, error)` and `func Load(path string) (*Credential, error)`.
  - `func (c *Credential) PublicKey() ed25519.PublicKey`, which returns a copy.
  - `func (c *Credential) Sign(message []byte) []byte`.
  - `func (c *Credential) Signer() crypto.Signer`, which returns a value whose only methods are `Public` and `Sign`, so a caller cannot reach the private key by a type assertion.
  - `func (c *Credential) Fingerprint() string`, which returns `SHA256:` and the unpadded standard base64 of the SHA-256 (Secure Hash Algorithm, 256 bits) hash of the 32-byte public key.
  - `func (c *Credential) String() string` and `GoString() string`, which both return `machine key ` and the fingerprint.
  - `var ErrKeyExists`, `ErrKeyMissing`, `ErrKeyExposed`, `ErrKeyNotOwned`, `ErrKeyNotRegular`, `ErrKeyInvalid`.
- **Behaviour:**
  - `Create` makes the key with `ed25519.GenerateKey(crypto/rand.Reader)`, encodes it with `x509.MarshalPKCS8PrivateKey` into one PEM block of type `PRIVATE KEY`, and writes it with `state.CreateExclusive`. An existing file gives `ErrKeyExists`, which wraps `os.ErrExist`, and the existing file is not changed.
  - `Load` opens the file with `os.O_RDONLY|syscall.O_NOFOLLOW` and checks the open file with `Stat`. A missing file gives `ErrKeyMissing`. A symbolic link or a file that is not regular gives `ErrKeyNotRegular`. Any bit of group or others gives `ErrKeyExposed`. An owner other than the user who runs the client gives `ErrKeyNotOwned`.
  - `Load` reads at most 4 kibibytes. Anything other than exactly one PEM block of type `PRIVATE KEY` holding an Ed25519 key gives `ErrKeyInvalid`.
  - No error message holds bytes of the key.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `credential_test.go` | `TestCreateWritesPrivateKeyFile` | After `Create(path)`, the file's mode is exactly `0600`, it holds one PEM block of type `PRIVATE KEY`, and `x509.ParsePKCS8PrivateKey` of it gives an `ed25519.PrivateKey` whose public half equals `PublicKey()`. |
| `credential_test.go` | `TestLoadReturnsSameKey` | `Load` after `Create` gives the same `PublicKey()` and the same `Fingerprint()`. |
| `credential_test.go` | `TestSignVerifies` | `ed25519.Verify(c.PublicKey(), m, c.Sign(m))` is true, and `Signer().Sign(rand.Reader, m, crypto.Hash(0))` gives a signature that also verifies. |
| `credential_test.go` | `TestSignerHidesPrivateKey` | `Signer()` is not an `ed25519.PrivateKey`, and a type assertion of it to `ed25519.PrivateKey` fails. |
| `credential_test.go` | `TestCreateRefusesExistingKey` | A second `Create` on the same path returns `ErrKeyExists`, and the file's bytes are unchanged. |
| `credential_test.go` | `TestLoadRefusesExposedKey` | After `os.Chmod` of the key file to `0640`, then `0604`, then `0660`, `Load` returns `ErrKeyExposed` each time. |
| `credential_test.go` | `TestLoadRefusesSymlink` | A symbolic link to a valid key file of mode `0600` gives `ErrKeyNotRegular`. |
| `credential_test.go` | `TestLoadRefusesFolder` | A folder at the path gives `ErrKeyNotRegular`. |
| `credential_test.go` | `TestLoadRefusesMissingAndInvalid` | A missing path gives `ErrKeyMissing`. A file of mode `0600` holding `hello`, one holding a PEM block of type `PUBLIC KEY`, and one holding an RSA key each give `ErrKeyInvalid`. |
| `credential_test.go` | `TestCredentialNeverPrintsKey` | For `%v`, `%+v`, `%#v` and `%s` of the credential, and for the text of every error the tests above produce, the output holds neither the base64 nor the hexadecimal text of the 32-byte seed of the key. `String()` starts with `machine key SHA256:`. |
| `credential_test.go` | `TestFingerprintFormat` | `Fingerprint()` is `SHA256:` and 43 characters of unpadded standard base64, and it equals that encoding of `sha256.Sum256(PublicKey())`. |

The check of the owner is not given a test, because a test cannot make a file owned by another user without the rights of the superuser. The reviewer reads that branch.

- **Acceptance:** `go test -race ./src/client/internal/enroll/...` passes. The break reviewer's report names the five mutations and the test that went red for each.

**Tasks of WS-2:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 3.1 | Write the making and loading of the key | `src/client/internal/enroll/credential.go`, `Create` and `Load` (target). Pattern: the design's section "Joining", step 3. |
| 3.2 | Write the signer that hides the key | `src/client/internal/enroll/credential.go`, `Signer` (target). |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The key file is private, and it is never replaced. | `go test -run 'TestCreateWritesPrivateKeyFile|TestCreateRefusesExistingKey' ./src/client/internal/enroll/` passes. |
| 2 | A key that others could read is refused. | `go test -run 'TestLoadRefusesExposedKey|TestLoadRefusesSymlink|TestLoadRefusesFolder' ./src/client/internal/enroll/` passes. |
| 3 | The key never reaches printed text. | `go test -run TestCredentialNeverPrintsKey ./src/client/internal/enroll/` passes. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The key reaches a log through a formatted print of the credential. | high | `String` and `GoString` return only the fingerprint, proven by `TestCredentialNeverPrintsKey`. |
| A link in the state folder points the key at a file that others can read. | high | `O_NOFOLLOW` and the check of the open file, proven by `TestLoadRefusesSymlink`. |
| An agent of the same user reads the key file. | medium | Accepted for now by D19, which makes one user on a machine a convenience. The design plans a user of the client's own later. |
| A second `join` replaces the key and erases the machine's identity. | medium | `CreateExclusive` never replaces a file, proven by `TestCreateRefusesExistingKey`. Plan `go-s3-client-link` decides what a second `join` does. |

**Verify:** the tests of item 3. Device check: none.

**Rollout detail:** step 3 of the rollout order.

**WS-3: The client can describe its machine and its harnesses**

This workstream reads what the design's table "What travels in wave 1" sends up as facts, harnesses found and heartbeat.
Item 4 reads the machine. Item 5 finds the harnesses.
Neither sends anything. Plan `go-s3-client-link` puts their values into the link's messages.
The types follow the fields of message `Client` in `src/contracts/proto/lgtm/v1/clients.proto`, so that the website shows the same meaning.

**Item 4. The facts and the load of the machine**

- **Files:** `src/client/internal/sysinfo/sysinfo.go`, `src/client/internal/sysinfo/parse.go`, `src/client/internal/sysinfo/sysinfo_linux.go`, `src/client/internal/sysinfo/sysinfo_darwin.go`, `src/client/internal/sysinfo/sysinfo_other.go`, `src/client/internal/sysinfo/parse_test.go`, `src/client/internal/sysinfo/sysinfo_linux_test.go`, `src/client/internal/sysinfo/testdata/`.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 2 and 5.
- **Work area:** `backend`. **Complexity:** medium, because it spans two operating systems and a binary decoder, and every parser is a pure function with fixture tests.
- **Scope:** `client/backend`.
- **Design statements:** the table "What travels in wave 1" (facts and heartbeat), the section "Knowing whether a machine is there" (load of the processor and memory), and C7.
- **Risky:** no.
- **Public names that others build against:**
  - `const OSLinux = "Linux"` and `const OSMacOS = "macOS"`.
  - `type Facts struct { Hostname string; OS string; Arch string; Cores uint32; MemoryBytes uint64; ClientVersion string }`.
  - `type Load struct { CPUPercent uint32; MemoryPercent uint32 }`.
  - `func ReadFacts(clientVersion string) (Facts, error)` and `func ReadLoad() (Load, error)`.
  - In `parse.go`, with no build line, for the tests: `parseMeminfo(text []byte) (total, available uint64, err error)`, `parseLoadavg(text []byte) (float64, error)`, `decodeDarwinLoadavg(raw []byte) (float64, error)`, `decodeDarwinUint64(raw []byte) (uint64, error)`, `cpuPercent(load1 float64, cores int) uint32`, `memoryPercent(total, available uint64) uint32`.
  - In `sysinfo_linux.go`: `var procDir = "/proc"`, which tests point at `testdata/`.
- **Behaviour:**
  - `Hostname` comes from `os.Hostname`. `Arch` is `runtime.GOARCH`. `Cores` is `runtime.NumCPU()`.
  - On Linux, memory comes from `MemTotal` and `MemAvailable` of `<procDir>/meminfo`, in kibibytes times 1,024. The load comes from the first field of `<procDir>/loadavg`.
  - On macOS, memory comes from `syscall.Sysctl("hw.memsize")`, the load from `syscall.Sysctl("vm.loadavg")`, and the memory in use from `100 - syscall.SysctlUint32("kern.memorystatus_level")`.
  - `syscall.Sysctl` drops one trailing zero byte. `decodeDarwinUint64` pads its input to 8 bytes, and `decodeDarwinLoadavg` pads to 24 bytes, before decoding as little-endian. `vm.loadavg` is three 32-bit values, 4 bytes of padding, and a 64-bit scale. The load of the last minute is the first value divided by the scale. A scale of 0 is an error.
  - `cpuPercent` is `round(load1 / cores × 100)`, at least 0 and at most 100. `memoryPercent` is `round((total − available) × 100 / total)`, and 0 when `total` is 0.
  - `sysinfo_other.go` has the build line `//go:build !linux && !darwin` and returns `errors.ErrUnsupported`.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `parse_test.go` | `TestParseMeminfo` | A fixture with `MemTotal: 16384000 kB` and `MemAvailable: 4096000 kB` gives 16,777,216,000 and 4,194,304,000 bytes. A fixture without `MemAvailable` gives an error. |
| `parse_test.go` | `TestParseLoadavg` | `0.52 0.58 0.59 1/1234 5678\n` gives 0.52. Empty text gives an error. |
| `parse_test.go` | `TestDecodeDarwinLoadavg` | 24 bytes holding the values 3072, 1024 and 512, 4 bytes of zeros and the scale 2048 give 1.5. The same bytes with the last zero byte removed also give 1.5. A scale of 0 gives an error, and 10 bytes give an error. |
| `parse_test.go` | `TestDecodeDarwinUint64` | The 8 little-endian bytes of 17,179,869,184 give that number. The same bytes with the last zero byte removed give the same number. 3 bytes give an error. |
| `parse_test.go` | `TestCPUPercent` | A load of 2.0 on 4 cores gives 50. A load of 9.0 on 4 cores gives 100. A load of 0 gives 0. A count of 0 cores gives 0. |
| `parse_test.go` | `TestMemoryPercent` | A total of 100 and 25 available give 75. A total of 0 gives 0. |
| `sysinfo_linux_test.go` | `TestReadFactsOnLinux` | With `procDir` set to `testdata/`, `OS` is `Linux`, `MemoryBytes` is the fixture's total, `Cores` equals `runtime.NumCPU()`, `ClientVersion` is the argument, and `Hostname` equals `os.Hostname()`. |
| `sysinfo_linux_test.go` | `TestReadLoadOnLinux` | With `procDir` set to `testdata/`, a fixture load of 2.0 and `runtime.NumCPU()` cores give `cpuPercent(2.0, NumCPU)`, and the memory fixture gives 75. |

- **Acceptance:** `go test ./src/client/internal/sysinfo/...` passes on Linux. `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./src/client/internal/sysinfo/...` exits 0.

**Item 5. The harness scan**

- **Files:** `src/client/internal/harness/registry.go`, `src/client/internal/harness/scan.go`, `src/client/internal/harness/registry_test.go`, `src/client/internal/harness/scan_test.go`.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 2 and 4.
- **Work area:** `backend`. **Complexity:** medium, because it starts child processes under a time limit, which follows a known pattern with clear tests.
- **Scope:** `client/backend`.
- **Design statements:** the table "What travels in wave 1" (harnesses found), the section "The client in wave 1" (the four harnesses), D8, R9 and C7.
- **Risky:** no. It touches no sign-in, key, token or ordering.
- **Public names that others build against:**
  - `type Name string` with `const Claude Name = "claude"`, `Codex Name = "codex"`, `Gemini Name = "gemini"`, `Cursor Name = "cursor"`. These are the values of `InstalledHarness.harness` in the contract.
  - `type Definition struct { Name Name; Programs []string; Version *regexp.Regexp }` and `func Definitions() []Definition`, in the order Claude, Codex, Gemini, Cursor.
  - `type Problem string` with `ProblemNone Problem = ""`, `ProblemNotFound = "not found"`, `ProblemTimedOut = "timed out"`, `ProblemFailed = "failed to run"`, `ProblemNoVersion = "no version in output"`.
  - `type Found struct { Harness Name; Installed bool; Version string; Path string; Problem Problem }`.
  - `type ScanOptions struct { SearchPath []string; Home string; Timeout time.Duration }`, `const DefaultTimeout = 3 * time.Second`, and `func DefaultScanOptions(getenv func(string) string) ScanOptions`, which splits `PATH` and reads `HOME`.
  - `func Scan(ctx context.Context, opts ScanOptions) []Found`.
- **Behaviour:**
  - The programs are `claude` for Claude, `codex` for Codex, `gemini` for Gemini, and `cursor-agent` then `agent` for Cursor.
  - The version pattern of Claude, Codex and Gemini is the first match of `\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?` in the first line of the output. The pattern of Cursor must match at the start of the trimmed first line: `^\d{4}\.\d{1,2}\.\d{1,2}(-[0-9A-Za-z]+)?`.
  - `Scan` returns exactly four values, in the order of `Definitions()`, and the four are scanned at the same time.
  - For each program name in order, and for each folder of `SearchPath` in order, a candidate is the folder joined with the name. A folder that is empty or not absolute is skipped. A candidate that is not a regular file, or has no execute bit, is skipped.
  - A candidate is run with `exec.CommandContext`, the path itself, and the one argument `--version`. It is never run through a shell. Its environment is exactly `PATH=<SearchPath joined by the list separator>`, `HOME=<Home>` and `LANG=C`. Its folder is `Home`. Its input is empty. At most 4 kibibytes of its output are read, and its error output is thrown away. `cmd.WaitDelay` is 1 second.
  - The first candidate whose output matches the pattern gives `Installed: true`, its `Path`, and the matched text as `Version`, cut to 64 characters.
  - When no candidate matches, `Installed` is false and `Version` is empty. `Problem` is `ProblemNotFound` when no candidate existed. Otherwise it is the problem of the first candidate: `ProblemTimedOut`, `ProblemFailed` for an exit code that is not 0, or `ProblemNoVersion`. `Path` is that first candidate.
  - A `Timeout` of 0 means `DefaultTimeout`.
- **How the tests fake the installed programs:** each test writes small scripts that start with `#!/bin/sh` into folders under `t.TempDir()`, with mode `0755`, and passes only those folders as `SearchPath` and a temporary folder as `Home`. The scan never reads the real `PATH`, so no real harness on the machine is run. The script is a fake of the harness. The scan still starts it directly, and the kernel reads its first line.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `registry_test.go` | `TestDefinitionsOrderAndPrograms` | The names are `claude`, `codex`, `gemini` and `cursor` in that order, and Cursor's programs are `cursor-agent` then `agent`. |
| `scan_test.go` | `TestScanFindsFakePrograms` | Fakes print `2.1.325 (Claude Code)`, `codex-cli 0.98.0`, `0.68.0` and, as `cursor-agent`, `2026.11.20-abc1234`. The result is installed for all four, with the versions `2.1.325`, `0.98.0`, `0.68.0` and `2026.11.20-abc1234`, and each `Path` is the fake's path. |
| `scan_test.go` | `TestScanEmptySearchPathFindsNothing` | With `SearchPath` empty, all four are not installed with `ProblemNotFound`. This holds even when the real machine has a harness on its `PATH`. |
| `scan_test.go` | `TestScanKeepsOnlyVersionText` | A fake `claude` prints `2.1.325 token=sk-planted-secret` and a second line `more`. `Version` is exactly `2.1.325`, and no field of the result holds `sk-planted-secret`. |
| `scan_test.go` | `TestScanPassesOnlyVersionArgument` | A fake writes its arguments to a file beside it. The file holds exactly one line, `--version`. |
| `scan_test.go` | `TestScanGivesMinimalEnvironment` | With `t.Setenv("LGTM_PLANTED_SECRET", "planted")`, a fake writes its environment to a file beside it. The file has no line with `planted`, and its `PATH` and `HOME` lines equal the given search path and home. |
| `scan_test.go` | `TestScanNeverUsesShell` | The fake `claude` sits in a folder named `bin $(touch INJECTED);x`. It is found and installed, and no file `INJECTED` exists in `Home` or in the test's folders afterwards. |
| `scan_test.go` | `TestScanSkipsRelativeFolders` | With `SearchPath` holding `.` and the current folder of the test changed to a folder with a fake `claude`, Claude is not installed. |
| `scan_test.go` | `TestScanSkipsNonExecutable` | A file `codex` of mode `0644` that prints a version gives not installed with `ProblemNotFound`. |
| `scan_test.go` | `TestScanTimesOut` | A fake `gemini` runs `exec sleep 30`. With `Timeout` 500 milliseconds, `Scan` returns within 3 seconds, and Gemini is not installed with `ProblemTimedOut`. |
| `scan_test.go` | `TestScanDoesNotWaitForGrandchild` | A fake `gemini` runs `sleep 30 &` and then `wait`. With `Timeout` 500 milliseconds, `Scan` returns within 3 seconds. |
| `scan_test.go` | `TestScanReportsFailureAndNoVersion` | A fake `claude` that exits with 1 gives `ProblemFailed`. A fake `codex` that prints `hello` gives `ProblemNoVersion`. Both are not installed. |
| `scan_test.go` | `TestCursorAgentNeedsCursorVersion` | A fake named `agent` that prints `1.4.2` gives Cursor not installed with `ProblemNoVersion`. A second folder, later in `SearchPath`, holds a fake `agent` that prints `2026.11.20-abc1234`, and then Cursor is installed from that second folder. |
| `scan_test.go` | `TestScanStopsOnCancel` | With a context cancelled before the call and four fakes that sleep, `Scan` returns within 2 seconds and reports none as installed. |

- **Acceptance:** `go test -race ./src/client/internal/harness/...` passes, and the run starts no program outside `t.TempDir()`.

**Tasks of WS-3:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 4.1 | Write the pure parsers and decoders | `src/client/internal/sysinfo/parse.go`, `parseMeminfo` and `decodeDarwinLoadavg` (target). Pattern: `src/contracts/proto/lgtm/v1/clients.proto`, message `Client`, fields `cpu_percent` and `memory_percent`. |
| 4.2 | Write the readers for Linux and macOS | `src/client/internal/sysinfo/sysinfo_linux.go` and `sysinfo_darwin.go`, `ReadFacts` and `ReadLoad` (target). |
| 5.1 | Write the definitions of the four harnesses | `src/client/internal/harness/registry.go`, `Definitions` (target). Pattern: `packages/engine/src/harnesses/recipes.ts`, `installRecipes`. |
| 5.2 | Write the scan | `src/client/internal/harness/scan.go`, `Scan` (target). Pattern: `packages/engine/src/harnesses/programs.ts`, `makeHarnessPrograms`, inner function `inspect`. |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The facts and the load are read on Linux. | `go test -run 'TestReadFactsOnLinux|TestReadLoadOnLinux' ./src/client/internal/sysinfo/` passes. |
| 2 | The values of macOS are decoded. | `go test -run 'TestDecodeDarwinLoadavg|TestDecodeDarwinUint64' ./src/client/internal/sysinfo/` passes, and `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./src/client/internal/sysinfo/` exits 0. |
| 3 | The scan finds fake harnesses and never touches the real ones. | `go test -run 'TestScanFindsFakePrograms|TestScanEmptySearchPathFindsNothing' ./src/client/internal/harness/` passes. |
| 4 | The scan runs no shell and leaks no secret. | `go test -run 'TestScanNeverUsesShell|TestScanKeepsOnlyVersionText|TestScanGivesMinimalEnvironment' ./src/client/internal/harness/` passes. |
| 5 | A program that never answers does not stop the scan. | `go test -run 'TestScanTimesOut|TestScanDoesNotWaitForGrandchild' ./src/client/internal/harness/` passes. |
| 6 | On a real Mac, the load and the memory are plausible. | A person runs a small `go run` program that prints `sysinfo.ReadLoad()` on a Mac while Activity Monitor is open, and both numbers are within 15 points of what Activity Monitor shows. This step belongs to phase 5, or it is logged as a gap when no Mac is available. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A harness prints a secret with its version, and the secret reaches the website. | high | Only the matched version text is kept, proven by `TestScanKeepsOnlyVersionText`. The child gets no variable of the client, proven by `TestScanGivesMinimalEnvironment`. |
| A program never ends, or a grandchild holds the output open. | medium | A time limit and `WaitDelay`, proven by `TestScanTimesOut` and `TestScanDoesNotWaitForGrandchild`. |
| Another program called `agent` is taken for Cursor. | medium | Cursor's pattern needs a version that starts with a date, proven by `TestCursorAgentNeedsCursorVersion`. |
| The formula for the load differs from what a person expects on macOS. | low | Decision 23 records the assumption, and done-when row 6 checks it on a Mac. |
| A test runs a real harness on the developer's machine. | medium | `Scan` reads only the given search path, proven by `TestScanEmptySearchPathFindsNothing`. |

**Verify:** the tests of items 4 and 5. Device check: done-when row 6 on a Mac, in phase 5.

**Rollout detail:** step 2 of the rollout order.

**WS-4: Nothing the client must send or has done is lost in a crash**

This workstream builds the two records on disk that the design's sections "Events come up" and "Commands go down" describe.
Item 6 is the format of one event file and the recovery of the outbox folder when it is opened.
Item 7 is the queue on top of it.
Item 8 is the receipts of commands.
The records hold opaque bytes, so they need no generated contract code. Plan `go-s3-client-link` puts the link's messages into them.

**Item 6. Event files and the acknowledged mark**

- **Files:** `src/client/internal/link/eventfile.go`, `src/client/internal/link/eventfile_test.go`.
- **Needs first:** item 2. Runs at the same time as items 3 and 8.
- **Work area:** `backend`. **Complexity:** medium, because it is a small binary format and one recovery function, with a fixture test for each state a crash can leave.
- **Scope:** `client/backend`.
- **Design statements:** the section "Events come up", and C11, whose "once, in order" stands on this recovery.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Skipping the check of the checksum makes `TestDecodeEventRejectsFlippedByte` fail.
  2. Computing the next number from the event files alone, without the mark, makes `TestRecoverNextAfterAllAcked` fail.
  3. Accepting a gap in the numbers makes `TestRecoverRefusesGap` fail.
  4. Not removing temporary files makes `TestRecoverRemovesTemps` fail.
- **Format of one event file:** 8 bytes of the text `LGTMEV01`; the event number as an unsigned 64-bit big-endian value; the length of the payload as an unsigned 32-bit big-endian value; the CRC-32C (cyclic redundancy check, Castagnoli polynomial) of the payload as an unsigned 32-bit big-endian value; then the payload. The header is 24 bytes.
- **Names that item 7 builds against (unexported, in package `link`):** `const eventSuffix = ".event"`, `const ackedFile = "acked"`, `func eventFileName(seq uint64) string`, `func encodeEvent(seq uint64, payload []byte) []byte`, `func decodeEvent(b []byte) (uint64, []byte, error)`, `type recovered struct { acked uint64; pending []uint64; sizes map[uint64]int; next uint64 }`, `func recoverOutbox(dir string) (recovered, error)`, `func writeAcked(dir string, n uint64) error`, `func readAcked(dir string) (uint64, error)`. Exported: `var ErrOutboxCorrupt`.
- **Behaviour:**
  - `eventFileName(42)` is `00000000000000000042.event`.
  - `decodeEvent` refuses a wrong magic text, a length that differs from the rest of the bytes, and a checksum that does not match, each with an error that wraps `ErrOutboxCorrupt`.
  - `writeAcked` writes the number in decimal and a newline through `state.WriteAtomic`. `readAcked` gives 0 when the file is missing, and an error that wraps `ErrOutboxCorrupt` when the text is not one decimal number and a newline.
  - `recoverOutbox` first calls `state.RemoveTemps`. It reads the mark. It removes every event file whose number is at most the mark, and then syncs the folder. It reads every other event file, and refuses with `ErrOutboxCorrupt` a file whose name and header disagree, a file that does not decode, and a gap between the mark plus 1 and the highest number. A file with another name is left alone. `next` is the larger of the mark and the highest number, plus 1.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `eventfile_test.go` | `TestEventFileName` | 1 gives `00000000000000000001.event`, and the largest 64-bit number gives 20 digits. |
| `eventfile_test.go` | `TestEncodeDecodeRoundTrip` | For payloads of 0 bytes, 1 byte and 1 mebibyte, decoding the encoding gives the same number and the same bytes. |
| `eventfile_test.go` | `TestDecodeEventRejectsFlippedByte` | Changing any one byte of the payload, the magic text or the length gives an error that wraps `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestDecodeEventRejectsTruncated` | Each length from 0 to the full length minus 1 gives an error. |
| `eventfile_test.go` | `TestAckedRoundTrip` | `writeAcked(dir, 17)` then `readAcked(dir)` gives 17. A missing file gives 0. A file holding `x\n` gives `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestRecoverEmptyFolder` | An empty folder gives a mark of 0, no pending event, and `next` 1. |
| `eventfile_test.go` | `TestRecoverDeletesAcked` | Event files 1 to 5 and a mark of 3 give pending 4 and 5, `next` 6, and files 1 to 3 are gone. |
| `eventfile_test.go` | `TestRecoverNextAfterAllAcked` | No event file and a mark of 9 give `next` 10. |
| `eventfile_test.go` | `TestRecoverRemovesTemps` | A file `.tmp-1234` holding half an event is gone, and it is not counted as pending. |
| `eventfile_test.go` | `TestRecoverRefusesGap` | Event files 1, 2 and 4 with no mark give `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestRecoverRefusesNameMismatch` | A file named `00000000000000000002.event` whose header says 3 gives `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestRecoverIgnoresOtherNames` | A file `notes.txt` stays and does not change the result. |

- **Acceptance:** `go test -race -run 'TestEventFile|TestEncode|TestDecode|TestAcked|TestRecover' ./src/client/internal/link/` passes. The break reviewer's report names the four mutations and the test that went red for each.

**Item 7. The outbox queue**

- **Files:** `src/client/internal/link/outbox.go`, `src/client/internal/link/outbox_test.go`, `src/client/internal/link/outbox_crash_test.go`.
- **Needs first:** item 6.
- **Work area:** `backend`. **Complexity:** medium, because the format and the recovery are done in item 6, and this item adds the numbers, the limits and one lock in memory, with a crash test.
- **Scope:** `client/backend`.
- **Design statements:** the section "Events come up" (numbered, on disk, within a limit, sent again from the first one not acknowledged), and C11.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Returning the number from `Append` before the event file is written makes `TestOutboxKillKeepsEveryNumberedEvent` fail.
  2. Removing acknowledged files before the mark is written makes `TestAckIsDurableBeforeDelete` fail.
  3. Dropping the oldest event instead of refusing makes `TestAppendRefusesWhenFull` fail.
  4. Taking the next number from the count of pending events makes `TestOutboxNeverReusesNumbers` fail.
- **Public names that others build against:**
  - `type OutboxOptions struct { MaxEvents int; MaxBytes int64; MaxEventBytes int }`, with `DefaultMaxEvents = 10000`, `DefaultMaxBytes = 64 << 20` and `DefaultMaxEventBytes = 1 << 20`. A field of 0 means its default.
  - `type Event struct { Seq uint64; Payload []byte }`.
  - `type Outbox struct` and `func OpenOutbox(dir string, opts OutboxOptions) (*Outbox, error)`.
  - `func (o *Outbox) Append(payload []byte) (uint64, error)`.
  - `func (o *Outbox) Read(from uint64, limit int) ([]Event, error)`.
  - `func (o *Outbox) Ack(upTo uint64) error`.
  - `func (o *Outbox) Acked() uint64`, `Next() uint64`, `Len() int`, `Bytes() int64`.
  - `func (o *Outbox) Changed() <-chan struct{}`.
  - `func (o *Outbox) Close() error`.
  - `var ErrOutboxFull`, `ErrEventTooLarge`, `ErrOutboxClosed`, and `ErrOutboxCorrupt` from item 6.
- **Behaviour:**
  - `OpenOutbox` runs `recoverOutbox` and keeps the mark, the pending numbers, their sizes and `next` in memory.
  - `Append` refuses a payload larger than `MaxEventBytes` with `ErrEventTooLarge`. It refuses with `ErrOutboxFull` when the pending count would pass `MaxEvents` or the pending bytes would pass `MaxBytes`. A refusal changes nothing on disk or in memory.
  - `Append` writes the event file for `next` with `state.WriteAtomic`, and only then takes the number, adds 1 to `next`, and returns the number. It then signals `Changed()` without blocking.
  - `Read(from, limit)` returns, in order, at most `limit` pending events whose number is at least the larger of `from` and the mark plus 1. A `limit` of 0 or less returns nothing. The payloads are read from disk.
  - `Ack(upTo)` does nothing when `upTo` is at most the mark. Otherwise it writes the mark with `writeAcked` first, then removes the event files up to `upTo`, syncs the folder, and sets `next` to the larger of `next` and `upTo + 1`. A failure to remove a file after the mark is written is not returned, because the next `Ack` or `OpenOutbox` removes it.
  - All methods are safe to call at the same time. After `Close`, every method returns `ErrOutboxClosed`.
  - Plan `go-s3-client-link` uses `Ack` beyond `Next()` when the server reports a higher last number than the client knows. See decision 17.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `outbox_test.go` | `TestAppendNumbersFromOne` | Three appends return 1, 2 and 3. `Read(0, 10)` returns the three payloads in order. |
| `outbox_test.go` | `TestReopenKeepsPending` | After three appends, `Close` and `OpenOutbox` again, `Read(0, 10)` returns the same three events, and the next append returns 4. |
| `outbox_test.go` | `TestReadFromFirstUnacked` | After five appends and `Ack(2)`, `Read(0, 10)` returns 3, 4 and 5. `Read(4, 10)` returns 4 and 5. `Read(0, 2)` returns 3 and 4. |
| `outbox_test.go` | `TestAckIsCumulativeAndIdempotent` | After `Ack(3)`, `Ack(2)` and `Ack(3)` return nil and change nothing. `Acked()` is 3. |
| `outbox_test.go` | `TestAckBeyondNextMovesCounter` | With nothing appended, `Ack(41)` makes the next append return 42, also after a reopen. |
| `outbox_test.go` | `TestOutboxNeverReusesNumbers` | After five appends, `Ack(5)`, a reopen, and one append, the number is 6. |
| `outbox_test.go` | `TestAckIsDurableBeforeDelete` | After five appends, the test replaces `removeFile` with a recorder that calls `readAcked(dir)` at its first call. `Ack(3)` makes that first call see the mark 3 already on disk, and the files of events 1 to 3 are gone afterwards. |
| `outbox_test.go` | `TestAppendRefusesWhenFull` | With `MaxEvents` 3, the fourth append returns `ErrOutboxFull`, `Len()` stays 3, and events 1 to 3 are unchanged. After `Ack(1)`, an append succeeds and returns 4. With `MaxBytes` 10, appends of 6 bytes and then 5 bytes give success and then `ErrOutboxFull`. |
| `outbox_test.go` | `TestAppendRefusesLargeEvent` | With `MaxEventBytes` 8, a payload of 9 bytes gives `ErrEventTooLarge`, and no file is written. |
| `outbox_test.go` | `TestChangedSignalsAppend` | After an append, a receive from `Changed()` succeeds within 1 second. |
| `outbox_test.go` | `TestConcurrentAppendsAreUnique` | 8 goroutines append 50 events each. The 400 numbers are 1 to 400 with no repeat, and `Read(0, 1000)` returns 400 events in order. |
| `outbox_test.go` | `TestClosedOutboxRefuses` | After `Close`, `Append`, `Read` and `Ack` return `ErrOutboxClosed`. |
| `outbox_crash_test.go` | `TestOutboxKillKeepsEveryNumberedEvent` | A child (helper `outbox-writer`) appends events in a loop. The payload of event `n` is the text `event-<n>` repeated up to a size between 1 byte and 64 kibibytes that depends on `n`. After each append returns, it prints `appended <n>` and acknowledges every tenth number. The parent kills it with `SIGKILL` after a random wait of 0 to 30 milliseconds, 30 times in a row, from a fixed seed that the test logs. After each kill, `OpenOutbox` succeeds. Every number that the child printed is either at most the mark or pending with its exact payload. The pending numbers run without a gap from the mark plus 1. The first number the next child prints is larger than every number printed before. |
| `outbox_crash_test.go` | `TestOutboxCrashAtEachWriteStep` | For each of the five steps of `state.WriteAtomic`, a child appends events 1 to 5, then exits with 86 at that step during event 6. After `OpenOutbox`, events 1 to 5 are pending with their exact payloads, event 6 is either pending whole or missing, and the next append returns 6 when it is missing and 7 when it is pending. |

The child of `TestOutboxCrashAtEachWriteStep` sets the hook with `state.SetCrashHookForTest` of item 2 only after events 1 to 5 are appended, so the hook fires during event 6 alone.
`removeFile` is an unexported variable of package `link`, `var removeFile = os.Remove`, which only tests replace.

- **Acceptance:** `go test -race -run 'TestAppend|TestReopen|TestRead|TestAck|TestOutbox|TestChanged|TestConcurrent|TestClosed' ./src/client/internal/link/` passes. The break reviewer's report names the four mutations and the test that went red for each.

**Item 8. Receipts of commands**

- **Files:** `src/client/internal/link/receipts.go`, `src/client/internal/link/receipts_test.go`, `src/client/internal/link/receipts_crash_test.go`.
- **Needs first:** item 2. Runs at the same time as items 3 and 6.
- **Work area:** `backend`. **Complexity:** medium, because it is one record with four states and a crash test, built on the safe write of item 2.
- **Scope:** `client/backend`.
- **Design statements:** the section "Commands go down" (a command that arrives twice runs once and the result is sent again, and a command past its expiry is not run), R6 and R9.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Writing the `started` receipt after the command runs instead of before makes `TestReceiptsKillNeverRunsTwice` fail.
  2. Leaving `started` receipts alone at open makes `TestOpenMarksStartedAsInterrupted` fail.
  3. Skipping the check of the expiry makes `TestBeginRefusesExpired` fail.
  4. Skipping the check of the identifier makes `TestBeginRejectsBadIDs` fail.
- **Public names that others build against:**
  - `type Command struct { ID string; Attempt uint32; ExpiresAt time.Time }`.
  - `type ReceiptState string` with `ReceiptStarted = "started"`, `ReceiptFinished = "finished"`, `ReceiptInterrupted = "interrupted"`, `ReceiptExpired = "expired"`.
  - `type Receipt struct { CommandID string; Attempt uint32; ExpiresAt time.Time; State ReceiptState; StartedAt time.Time; FinishedAt time.Time; Result []byte }`.
  - `type Decision int` with `DecisionRun Decision = 1`, `DecisionReplay Decision = 2`, `DecisionRunning Decision = 3`.
  - `type ReceiptsOptions struct { Now func() time.Time; Keep time.Duration; MaxResultBytes int }`, with `DefaultReceiptKeep = 24 * time.Hour` and `DefaultMaxResultBytes = 1 << 20`. A field left empty means its default, and `Now` defaults to `time.Now`.
  - `type Receipts struct` and `func OpenReceipts(dir string, opts ReceiptsOptions) (*Receipts, error)`.
  - `func (r *Receipts) Begin(c Command) (Decision, Receipt, error)`.
  - `func (r *Receipts) Finish(id string, result []byte) (Receipt, error)`.
  - `func (r *Receipts) Get(id string) (Receipt, bool, error)`.
  - `func (r *Receipts) Prune() (int, error)`.
  - `func (r *Receipts) Close() error`.
  - `var ErrInvalidCommand`, `ErrNotStarted`, `ErrResultTooLarge`, `ErrReceiptsClosed`.
- **Format of one receipt file:** `receipts/<command id>.json`, mode `0600`, written with `state.WriteAtomic`. It is a JSON object with `version` (always 1), `command_id`, `attempt`, `expires_at`, `state`, `started_at`, `finished_at` and `result`. The times are text in the form of RFC 3339 (Request for Comments 3339) with nanoseconds, in UTC (Coordinated Universal Time). `finished_at` is left out while it is zero. `result` is the base64 of the bytes, and it is left out when empty.
- **Behaviour:**
  - `Begin` refuses with `ErrInvalidCommand` an `ID` that is not 36 characters of lowercase hexadecimal digits and hyphens in the canonical form of a universally unique identifier (UUID), an `Attempt` of 0, and a zero `ExpiresAt`. It writes nothing then.
  - When a receipt exists in the state `started` in this process, `Begin` returns `DecisionRunning` and that receipt. The caller sends nothing now, because `Finish` sends the result later.
  - When a receipt exists in any other state, `Begin` returns `DecisionReplay` and the receipt, and it writes nothing.
  - When no receipt exists and `Now()` is after `ExpiresAt`, `Begin` writes a receipt in the state `expired` and returns `DecisionReplay` with it.
  - Otherwise `Begin` writes a receipt in the state `started`, and only then returns `DecisionRun`.
  - `Finish` refuses with `ErrNotStarted` an identifier whose receipt is not `started` in this process, and with `ErrResultTooLarge` a result above `MaxResultBytes`. Otherwise it writes the receipt as `finished` with the result and the time, and only then returns it.
  - `OpenReceipts` removes temporary files. It rewrites every receipt in the state `started` as `interrupted`, with an empty result. It rewrites a file that does not decode as `interrupted`, with the identifier taken from its name, when the name is a valid identifier. It then runs `Prune`.
  - `Prune` removes every receipt that is not `started` and whose latest time of `ExpiresAt`, `StartedAt` and `FinishedAt` is more than `Keep` before `Now()`. It returns the number removed.
  - A replayed result is the result of the attempt that ran it. Plan `go-s3-client-link` labels the reply with the attempt of the command that just arrived, because the server ignores a result for an older attempt.
  - All methods are safe to call at the same time. After `Close`, every method returns `ErrReceiptsClosed`.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `receipts_test.go` | `TestBeginRunsNewCommand` | A new valid command gives `DecisionRun`, and the file `<id>.json` exists with the state `started` before `Begin` returns. |
| `receipts_test.go` | `TestSecondArrivalReplaysResult` | After `Begin` and `Finish(id, "ok")`, a second `Begin` with attempt 2 gives `DecisionReplay`, the state `finished`, the result `ok` and the attempt 1. |
| `receipts_test.go` | `TestArrivalWhileRunning` | A second `Begin` before `Finish` gives `DecisionRunning`. |
| `receipts_test.go` | `TestBeginRefusesExpired` | With `Now` fixed at noon and `ExpiresAt` one second before noon, `Begin` gives `DecisionReplay` with the state `expired`, and a second `Begin` gives the same. |
| `receipts_test.go` | `TestFinishedSurvivesReopen` | After `Finish`, `Close` and `OpenReceipts`, `Begin` gives `DecisionReplay` with the same result. |
| `receipts_test.go` | `TestOpenMarksStartedAsInterrupted` | After `Begin` with no `Finish`, `Close` and `OpenReceipts`, `Get` gives the state `interrupted`, and `Begin` gives `DecisionReplay`, not `DecisionRun`. `Finish` gives `ErrNotStarted`. |
| `receipts_test.go` | `TestOpenTreatsUnreadableReceiptAsInterrupted` | A file `<valid id>.json` holding `{` gives the state `interrupted` after `OpenReceipts`, and `Begin` with that identifier never gives `DecisionRun`. |
| `receipts_test.go` | `TestBeginRejectsBadIDs` | `../machine.key`, `a/b`, an uppercase UUID, a UUID without hyphens, and the empty text each give `ErrInvalidCommand`, and no file is written in the folder or beside it. An attempt of 0 and a zero expiry give the same. |
| `receipts_test.go` | `TestFinishRefusesLargeResult` | With `MaxResultBytes` 4, a result of 5 bytes gives `ErrResultTooLarge`, and the receipt stays `started`. |
| `receipts_test.go` | `TestPruneRemovesOldReceipts` | With `Keep` one hour, a finished receipt whose times are all two hours before `Now` is removed, and one whose expiry is in 30 minutes stays. A `started` receipt is never removed. `Prune` returns 1. |
| `receipts_test.go` | `TestReceiptFileFormat` | The written file decodes as JSON with exactly the fields of the format above, `version` is 1, the times end in `Z`, and the mode is `0600`. |
| `receipts_test.go` | `TestConcurrentBeginRunsOnce` | 16 goroutines call `Begin` with the same command. Exactly one gets `DecisionRun`. |
| `receipts_crash_test.go` | `TestReceiptsKillNeverRunsTwice` | A child (helper `receipts-runner`) loops over identifiers made from a counter. For each, it calls `Begin`, prints `begun <id>` after `DecisionRun`, calls `Finish` with the result `result-<id>`, and prints `finished <id>` after it returns. The parent kills it with `SIGKILL` after a random wait of 0 to 30 milliseconds, 30 times in a row, from a fixed seed that the test logs. After each kill, every identifier printed as `finished` has the state `finished` and the exact result. Every identifier printed as `begun` has a receipt in the state `finished` or `interrupted`, never none. `Begin` of each printed identifier never gives `DecisionRun`. |
| `receipts_crash_test.go` | `TestReceiptsCrashAtEachWriteStep` | For each step of `state.WriteAtomic`, a child exits with 86 at that step while it writes the `started` receipt. After `OpenReceipts`, `Begin` of that identifier gives either `DecisionRun`, when the receipt never reached its name, or `DecisionReplay` with `interrupted`. The child never printed `begun` for it. The same is done for the step of `Finish`, and the receipt is then either `interrupted` or `finished` with the exact result. |

- **Acceptance:** `go test -race -run 'TestBegin|TestSecond|TestArrival|TestFinish|TestOpen|TestPrune|TestReceipt|TestConcurrentBegin' ./src/client/internal/link/` passes. The break reviewer's report names the four mutations and the test that went red for each.

**How the crash tests reach a step of the safe write from another package**

The step hook `crashHook` of item 2 is unexported in package `state`, so a test of package `link` cannot set it.
Item 2 therefore also provides `state.SetCrashHookForTest` in `src/client/internal/state/testhook.go`.
It is the only exported name of the plan that exists for tests.
`TestOutboxCrashAtEachWriteStep` and `TestReceiptsCrashAtEachWriteStep` call it in their child processes.

**Tasks of WS-4:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 6.1 | Write the format of an event file | `src/client/internal/link/eventfile.go`, `encodeEvent` and `decodeEvent` (target). Pattern: the design's section "Events come up". |
| 6.2 | Write the recovery of the outbox folder | `src/client/internal/link/eventfile.go`, `recoverOutbox` (target). |
| 7.1 | Write the queue | `src/client/internal/link/outbox.go`, `OpenOutbox`, `Append` and `Ack` (target). Pattern: `floorplan/plan.json`, entry "Events waiting to be sent". |
| 8.1 | Write the receipts | `src/client/internal/link/receipts.go`, `OpenReceipts`, `Begin` and `Finish` (target). Pattern: the design's section "Commands go down". |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Every event that got a number survives a kill, whole and in order. | `go test -run TestOutboxKillKeepsEveryNumberedEvent ./src/client/internal/link/` passes. |
| 2 | A crash at any step of a write leaves an event whole or missing. | `go test -run TestOutboxCrashAtEachWriteStep ./src/client/internal/link/` passes. |
| 3 | A number is never given to two events. | `go test -run 'TestOutboxNeverReusesNumbers|TestAckIsDurableBeforeDelete|TestConcurrentAppendsAreUnique' ./src/client/internal/link/` passes. |
| 4 | A full outbox refuses and loses nothing. | `go test -run TestAppendRefusesWhenFull ./src/client/internal/link/` passes. |
| 5 | A command that arrives twice runs once and gets its first result. | `go test -run 'TestSecondArrivalReplaysResult|TestConcurrentBeginRunsOnce' ./src/client/internal/link/` passes. |
| 6 | No command runs twice across a kill. | `go test -run 'TestReceiptsKillNeverRunsTwice|TestReceiptsCrashAtEachWriteStep|TestOpenMarksStartedAsInterrupted' ./src/client/internal/link/` passes. |
| 7 | A command past its expiry is not run, and a bad identifier never reaches the disk. | `go test -run 'TestBeginRefusesExpired|TestBeginRejectsBadIDs' ./src/client/internal/link/` passes. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| An event is sent before it is on disk, and a crash gives its number to another event that the server then drops. | high | `Append` returns the number only after the safe write, proven by `TestOutboxKillKeepsEveryNumberedEvent` and by the first mutation of item 7. |
| Acknowledged files are removed before the mark, and a crash lets numbers start again lower. | high | The mark is written first, proven by `TestAckIsDurableBeforeDelete` and `TestOutboxNeverReusesNumbers`. |
| A command runs twice after a crash. | high | The `started` receipt is on disk before the command runs, and a restart turns it into `interrupted`, proven by `TestReceiptsKillNeverRunsTwice`. |
| An identifier from the server names a file outside the receipts folder. | high | Only the canonical form of a UUID is accepted, proven by `TestBeginRejectsBadIDs`. |
| The client's clock is behind the server's, so an expired command still runs. | medium | Decision 21 records the assumption. `ReceiptsOptions.Now` lets plan `go-s3-client-link` pass a corrected clock. |
| A long outage fills the outbox. | medium | The outbox refuses, and the producer decides. The limits are options. Wave 2 revisits them when agents produce events. |
| Two disk syncs for each event are too slow for wave 2. | low | Decision 16 keeps the interface, so wave 2 can change the storage behind it. |

**Verify:** the tests of items 6, 7 and 8, all under `-race`. Device check: none. The kill tests run on Linux, and phase 5 runs them on a Mac when one is available.

**Rollout detail:** steps 3 and 4 of the rollout order.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe:

1. The six work items of the brief (`plan.md` of 2026-11-25).
2. The four points the brief asks the planner to settle.
3. The four points of the task file `.foundry/go-rewrite/tasks/plan-go-s1-client-local.md` under "Specific to this plan".
4. The statements of the design that this plan builds on, as listed in `scouting.md`: D2, D7, D8, D16, D19, R1, R6, R9, R10, C7, C10, C11, and the sections "Joining" step 3, "Commands go down", "Events come up", "What travels in wave 1", "Knowing whether a machine is there" and "The client in wave 1".
5. Point 4 of the epic's section "What the teams must agree on".

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the program and its settings | WS-1, item 1 | mapped |
| Brief item 2, the key of the machine | WS-2, item 3, on top of WS-1 item 2 | mapped |
| Brief item 3, the facts of the machine | WS-3, item 4 | mapped |
| Brief item 4, the harness scan | WS-3, item 5 | mapped |
| Brief item 5, events that wait for the server | WS-4, items 6 and 7 | mapped |
| Brief item 6, receipts of commands | WS-4, item 8 | mapped |
| Brief point 1, a full buffer | WS-4, item 7; decision 14 | mapped |
| Brief point 2, the size and the flushing of the buffer | WS-4, item 7; decision 15 | mapped |
| Brief point 3, the load on macOS without a shell | WS-3, item 4; decision 23 | mapped |
| Brief point 4, receipts in their own file | WS-4, item 8; decision 25 | mapped |
| Task point 1, the layout of the state folder | Implementation details, "The layout of the state folder"; WS-1 item 2; decision 9 | mapped |
| Task point 2, faking the installed programs | WS-3, item 5, "How the tests fake the installed programs"; decision 28 | mapped |
| Task point 3, surviving a crash in a write | WS-1 item 2 and WS-4 items 6 to 8, the crash tests; decision 29 | mapped |
| Task point 4, depending on `go-s0-contract` only for `go.mod` | Rollout order step 1; decisions 18 and 24 | mapped |
| D2, Go | every item | mapped |
| D7, a machine proves itself with a key pair | WS-2, item 3 | mapped |
| D8, ACP only | WS-3, item 5. The scan reports the plain programs, and wave 2 starts their ACP forms. | mapped |
| D16, no web server | WS-1, item 1. The program has no command that serves. | mapped |
| D19, one user on a machine | WS-1 item 2 and WS-2 item 3, as the accepted risk of an agent of the same user | mapped |
| R1, one folder for each part | every item keeps its own folder under `src/client/internal/` | mapped |
| R6, commands through the stored commands | WS-4, item 8 is the client's half | mapped |
| R9, text from outside is data, and secrets never reach a log | WS-2 item 3, WS-3 item 5, WS-4 item 8 | mapped |
| R10, real tests | Guardrails. No test here needs a database, and no test replaces a part of the client with a stand-in. | mapped |
| C7, facts, load and harnesses shown | WS-3, items 4 and 5, for the client's half | mapped |
| C10, an unknown key is refused | WS-2, item 3 makes the key that the server knows | mapped |
| C11, buffered events arrive once, in order | WS-4, items 6 and 7, for the client's half | mapped |
| Section "Joining", step 3 | WS-2, item 3 | mapped |
| Section "Commands go down" | WS-4, item 8 | mapped |
| Section "Events come up" | WS-4, items 6 and 7 | mapped |
| Section "What travels in wave 1" | WS-3, items 4 and 5 | mapped |
| Section "Knowing whether a machine is there" | WS-3, item 4, the load | mapped |
| Section "The client in wave 1" | WS-1, items 1 and 2; WS-3, item 5 | mapped |
| Epic agreement point 4, the layout of the state folder | Implementation details, "The layout of the state folder" | mapped |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the server and the client are written from scratch in Go. | Every item is Go under `src/client/`. Nothing is ported from `packages/`, which is read only as a pattern. |
| `AGENTS.md`, "Where things are": `packages/` is a reference, and new code does not go there. | No file under `packages/` is named in any item. |
| The epic's table of teams: the Client team edits only `src/client/`. | Every file of every item is under `src/client/`. The cross build test sits in `src/client/cmd/lgtm-client/` because `scripts/` belongs to the Contract team. |
| `AGENTS.md`, habit 1: credit copied code. | No code is copied. The old engine gives ideas only. |
| `AGENTS.md`, habit 2: write down what you find. | The result file lists the points for other teams. Issues that come up in the build go into `docs/issues.md`. |
| `AGENTS.md`, "Writing style for documents", enforced by `scripts/check-private.mjs`. | The plan names no home folder of a real user, no network address and no quotation. Paths start with `~/` or are relative to the repository. Acronyms are expanded the first time. |
| The design's rule 9: secrets never reach a log. | Item 3 never prints the key. Item 5 keeps only the version text. |
| The design's rule 10: tests run against a real PostgreSQL. | This plan stores nothing in a database, so no test needs one. No test replaces the file system or a process with a stand-in: the crash tests kill real processes and read real files. |
| The rule for planners: name no model and no harness for a seat. | The four harnesses appear only as data that item 5 looks for. Decision 33 records this. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch. This section names none, by the owner's rule of 2026-10-16.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. The program and its settings | backend | client/backend | low | A fixed set of commands and a small reader of one file, with no design choice left. | no | `go.mod` |
| 2. The state folder and safe writes | backend | client/backend | medium | A known method of writing in two small files, made risky by ordering and covered by named crash tests. | no | `go.mod` |
| 3. The key of the machine | backend | client/backend | medium | The standard library does the cryptography. The work is file handling with one test for each refusal. | no | 2 |
| 4. The facts and the load | backend | client/backend | medium | Two operating systems and a binary decoder, all pure functions with fixtures. | no | `go.mod` |
| 5. The harness scan | backend | client/backend | medium | Child processes under a time limit, a known pattern with clear tests. | no | `go.mod` |
| 6. Event files and the mark | backend | client/backend | medium | A small binary format and one recovery function, with a fixture for each crash state. | no | 2 |
| 7. The outbox queue | backend | client/backend | medium | Numbers, limits and one lock on top of item 6, with a kill test. | no | 6 |
| 8. Receipts of commands | backend | client/backend | medium | One record with four states on top of item 2, with a kill test. | no | 2 |

No unit is rated high. The brief rated items 2 and 5 high, and both were split by decision 7.
No unit changes an entry point that outsiders can reach, so no unit needs an attack pass.
The scope file `.plans/scopes/client/backend/scope.json` does not exist yet. The orchestrator writes the scope files, as the shared rules for planners say.

Load: each unit has one test writer, one builder and one code reviewer, and the reader of the third vendor beside the code review. The five risky units (2, 3, 6, 7 and 8) add one break reviewer each. That is 8 test writers, 8 builders, 8 code reviewers, 8 readers and 5 break reviewers, 37 launches in all. With builders alternating between the two main vendors, one of them takes 15 launches and the other 14, and the third vendor takes the 8 reader seats. The orchestrator checks these counts against each vendor's weekly meter at the start of sprint 1, as the epic says. If one vendor would run out, the units of WS-3, which share nothing with the others, are the first to move later in the sprint.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 0 | none yet | The planner stopped after the self-check, as the shared rules for planners say. The orchestrator runs the review rounds with other vendors. |

Known gaps from the self-check, for the reviewers:

- Gate 1: row 5 of `decisions.md` is open. It is the owner's decision on the cut of wave 1 into sprints, and the planner cannot close it. Decision 34 records that the plan builds on the cut as written.
- Gate 7: no attack round has run yet.
- Gate 10: the scope file `client/backend` does not exist yet, and the remaining weekly allowance of each vendor is not known to the planner.
- The owner check of item 3 has no test, because a test cannot make a file owned by another user without the rights of the superuser.
- macOS code is compiled and vetted on Linux, but its tests run only when a Mac is available in phase 5.

</details>
