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
This plan builds the pieces it uses on its own machine, so that plan `go-s3-client-link` in sprint 3 only has to connect them to the server.

When this plan is done, the program `lgtm-client` builds for Linux and for macOS, and it keeps its state in a folder that only its user can read.
It can make the machine's key pair, read the facts and the load of the machine, and find which of the four coding-agent programs of the design are installed.
It keeps a store on disk with the events that wait for the server and a receipt of each command it ran.

The store carries two promises of the design: no event is lost, and no command runs twice.
It is the embedded database `go.etcd.io/bbolt`, and each change to it is one transaction that is written whole and synced before it returns.

Review round 3 found that the plan had grown to about 1,000 lines, mostly with detail that the design does not ask for.
By rulings of the orchestrator of 2026-11-26, this version removes that detail, and restoring the state folder or the store to an earlier copy is outside wave 1.
Two choices are left for the owner, and the section "For the owner" describes them.

## Key decisions

**Every event is on disk before it gets a number that can be sent.**
Adding an event takes its number and stores the event in one transaction, and the number is returned only after the transaction is on disk.

**An acknowledgement removes every event up to its number.**
The server proves itself before it acknowledges anything, and it acknowledges only what it applied.
A number above the highest number the store handed out is refused, and the client stops and reports.

**A command runs at most once, even across a crash.**
The client writes a receipt that says "started" before it runs a command, and the result when the command ends.
A receipt that is still "started" from an earlier run of the program counts as interrupted, and the command is not run again.
A finished receipt is removed when the machine's clock is more than 24 hours past the command's expiry, as the design and the contract say (UNREVIEWED FIX, ruling 1 of round 4).

**The harness scan runs each known program with `--version`, and nothing more.**
It runs no shell, passes a clean environment, stops a program after 3 seconds, and accepts only a version line of the exact form.

**The plan needs one module outside Go's standard library**, the store `go.etcd.io/bbolt` at v1.4.3. The builder of item 6 adds it to `go.mod` in the same change as its first import (UNREVIEWED FIX, ruling 4 of round 4).

Full log → `decisions.md`

## Workstreams

### The client program starts and keeps its state folder private

The program gets its three commands for sprint 1, and its state folder is private, held by one client at a time, and written so that a crash leaves the old file or the new one.
When it is done, `go test ./src/client/...` passes, and a test builds the program for Linux and macOS.

### The machine has a key that never leaves it

The client makes an Ed25519 key pair, the kind the design names, keeps the private key in a file only its user can read, and never overwrites or prints it.
When it is done, the key tests pass, and a reviewer who removes the permission check sees them fail.

### The client can describe its machine and its harnesses

The client reads the facts and the load that the Machines page shows, and the version of each installed harness, under a time limit and with an exact form of the version line.
When it is done, the tests pass with fake programs, and no real harness is run.

### Nothing the client must send or has done is lost in a crash

The client keeps its events in the store until the server acknowledges them, and a receipt of each command in the same store.
When it is done, the kill tests pass, and a reviewer who writes a receipt after the command runs sees them fail.

## Rollout order

1. Wait for `go.mod` from plan `go-s0-contract`. Item 6 adds `go.etcd.io/bbolt` itself, and the orchestrator compares that change of `go.mod` and `go.sum` with this plan before it commits (UNREVIEWED FIX, architect-r4 09).
2. Build items 1, 2, 4 and 5 at the same time, because they share no file.
3. Build items 3 and 6 when item 2 is merged, because both use its state folder. No item needs code that a later item writes.
4. Run the whole suite, the build for macOS and the post-build checks on the merged work, in phase 5 of the build loop.
5. Hand the layout of the state folder and the fold "What plan go-s3-client-link must do" to that plan's planner before it freezes.

Nothing in this plan is hard to reverse, and no step needs the owner.

## For the owner

Two choices are left that the design does not settle, and neither blocks sprint 1, because the plan takes the safe side until the owner rules.

**What happens when the store file does not open.**
A damaged store file stops the client, and nothing is deleted. A new empty store would hand out numbers that the server already has, so it cannot simply start again.

1. Keep refusing, as the plan does now. A person removes the state folder and joins the machine again as a new machine. This costs nothing.
2. Add a command that does that removal after the person confirms it. This costs one command in plan `go-s3-client-link`.

**How the owner finds a machine whose join stopped half-way.**
When the server's reply to a join was lost and the token then expired, the website shows a waiting machine that the owner must remove, and two waiting machines can have the same host name.

1. Remove every waiting machine with that host name, and join again. This costs nothing, and it can remove an unrelated machine.
2. The client prints the fingerprint of its key, and the Machines page shows the fingerprint of each waiting machine. This costs one field on the website and in the contract.
3. A new token with the same key replaces the waiting machine on the server. This costs one rule in the server and in the contract.

The client trusts its own clock to remove old receipts. A wrong clock on the machine is an accepted limit of wave 1, by ruling 1 of round 4.
Round 4 was the last round without the owner, and its fixes are marked UNREVIEWED FIX, each with a post-build check.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**The rules that every item of this plan keeps**

- The team edits only `src/client/`. The module path is `github.com/qnlbnsl/lgtm-studio`, so a package is imported as `github.com/qnlbnsl/lgtm-studio/src/client/internal/<name>`.
- Only the standard library of Go is imported, except `go.etcd.io/bbolt` v1.4.3 in item 6. No test needs a server, a database server or the network, and every test passes under `go test -race`.
- The client builds for Linux and macOS with `CGO_ENABLED=0`. A file that only one system compiles carries a build line, `//go:build linux` or `//go:build darwin`.
- A test that needs a crash runs the test binary again as a child, with `os.Executable()`, the argument `-test.run=^<helper>$`, and the variable `LGTM_TEST_HELPER` naming its mode. The helpers are `TestHelperStateProcess` in item 2 and `TestHelperStoreProcess` in item 6, and each returns at once when the variable names none of its modes.
- Every work item has the scope `client/backend` and the work area `backend`. No item has a screen, so no item has a design artifact.

**The layout of the state folder**

Plan `go-s3-client-link` reads this layout. It is point 4 of the epic's section "What the teams must agree on".

| Path inside the folder | Kind and mode | Format | Written by | Read by |
|---|---|---|---|---|
| (the folder) | folder, `0700` | | item 2, `state.Open` | every item |
| `lock` | file, `0600` | Empty. A client holds an exclusive `flock` on it while it uses the folder. | item 2 | item 2 |
| `settings.json` | file, `0600` | JSON (JavaScript Object Notation): `{"version":1,"repositories_folder":"~/…","agent_limit":4}`. Both fields after `version` are optional. | the owner | item 1, `config.LoadSettings` |
| `machine.key` | file, `0600` | One PEM (Privacy-Enhanced Mail) block of type `PRIVATE KEY` with the Ed25519 key in PKCS #8 (Public-Key Cryptography Standards number 8) form, and at most one newline after it. | item 3, `enroll.Create` | item 3, `enroll.Load` |
| `identity.json` | file, `0600` | JSON that plan `go-s3-client-link` defines and writes last in a join. The user-flow test `deploy/test/checks.sh` reads its fields `clientPublicKey` and `serverPublicKey`. A key without this file means a join that did not finish. | plan `go-s3-client-link`, through `state.WriteAtomic` | plan `go-s3-client-link`, the user-flow test |
| `client.db` | file, `0600` | The store of item 6, with the buckets `meta`, `events` and `receipts`. | item 6 | item 6 |
| `repositories/` | folder, `0700` | The default folder for clones. Item 1 only names it, and wave 2 fills it. | wave 2 | wave 2 |
| `.tmp-<random>` and `.tmp-<random>.db` | file, `0600` | An unfinished write or an unfinished new store. `state.Open` removes it while it holds the lock. | items 2 and 6 | nobody |

The folder is `LGTM_CLIENT_STATE_DIR` when that variable is set and not empty, and it must then be an absolute path.
Otherwise it is `$HOME/.local/state/lgtm-client`, the folder the design names.
Nothing else is read, and the installer sets `LGTM_CLIENT_STATE_DIR` for the service, by the orchestrator's ruling 6 of 2026-11-26.

**The statements of the design, by number**

The numbering counts the design file as it is on `master`, and `scouting.md` lists every statement with its line.
D2 is the Go rewrite. D7 is the key pair of a machine. D8 is the Agent Client Protocol (ACP) only. D16 is "a client starts no web server". D19 is one user on a machine.
R1 is one folder for each part. R6 is commands through the link's stored commands. R9 is "text from an agent is data, and secrets never reach a log". R10 is real tests.
C7 is "the machine shows its facts, its load and its harnesses". C10 is "an unknown key is refused". C11 is "buffered events arrive once, in order".

**WS-1: The client program starts and keeps its state folder private**

Item 1 is the program and its settings. Item 2 is the state folder and the safe write of a plain file.

**Item 1. The program and its settings**

- **Files:** `src/client/cmd/lgtm-client/main.go`, `main_test.go` and `build_test.go` in the same folder, and `src/client/internal/config/config.go` and `config_test.go`.
- **Needs first:** `go.mod`. Runs at the same time as items 2, 4 and 5.
- **Complexity:** low, because it is a fixed set of commands and a small file reader with no choice left.
- **Design statements:** D2, D16 (no command serves the web), D19, R1, and the section "The client in wave 1".
- **Risky:** no.
- **Public names:** in `main`, `func run(args []string, stdout, stderr io.Writer) int` and `var version = "0.0.0-dev"`, which an installer sets with `-ldflags "-X main.version=<v>"`. In `config`, `const StateDirVariable = "LGTM_CLIENT_STATE_DIR"`, `func StateDir(getenv func(string) string) (string, error)`, `type Settings struct { RepositoriesFolder string; AgentLimit uint32 }`, `func DefaultSettings(stateDir string) Settings`, `func LoadSettings(path, home, stateDir string) (Settings, error)`, `ErrNoStateDir` and `ErrSettingsInvalid`. Writer: item 1. Readers: plan `go-s3-client-link` and wave 2.
- **Behaviour:**
  - `run` with `version` prints the version and a newline and returns 0. With `join` or `run`, it prints one sentence to `stderr` that names the command and says it is not built yet, and returns 1. With no argument or an unknown one, it prints the usage to `stderr` and returns 2. `StateDir` follows the rule of the layout. A relative `LGTM_CLIENT_STATE_DIR` is an error. Without the variable, `HOME` must be an absolute path, or the result is `ErrNoStateDir`. `XDG_STATE_HOME` is not read (ruling 6).
  - `DefaultSettings(stateDir)` gives the folder `<stateDir>/repositories` and the agent limit 4. `LoadSettings` gives the defaults when the file does not exist. It refuses with `ErrSettingsInvalid` text that is not JSON, a `version` other than 1, an unknown field, a folder that is neither absolute nor starts with `~/`, and an agent limit outside 1 to 64. A folder that starts with `~/` is expanded against `home`.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `main_test.go` | `TestVersionPrintsVersion` | `run([]string{"version"}, …)` returns 0, and `stdout` is the value of `version` and a newline. |
| `main_test.go` | `TestJoinAndRunAreNotBuiltYet` | For `join` and `run`, the return is 1, `stdout` is empty, and `stderr` holds the command's name and the words `not built yet`. |
| `main_test.go` | `TestUnknownCommandPrintsUsage` | For no argument and for `serve`, the return is 2, and `stderr` names `join`, `run` and `version`. |
| `build_test.go` | `TestBuildsForLinuxAndMacOS` | Skipped under `-short`. For `linux` and `darwin` with `amd64` and `arm64`, `CGO_ENABLED=0 go build ./src/client/...` and `go build -o <temp>/lgtm-client ./src/client/cmd/lgtm-client` exit 0, and the program file exists. The test fails, and does not skip, when `go` is not on the `PATH`. |
| `config_test.go` | `TestStateDirRule` | With `LGTM_CLIENT_STATE_DIR=/a/b`, `XDG_STATE_HOME=/x` and `HOME=/h`, the result is `/a/b`. Without the first, it is `/h/.local/state/lgtm-client` (ruling 6). `LGTM_CLIENT_STATE_DIR=rel` gives an error, and `HOME=rel` or no variable at all gives `ErrNoStateDir`. |
| `config_test.go` | `TestLoadSettings` | A missing file gives `DefaultSettings(stateDir)`, whose folder is `<stateDir>/repositories`. `{"version":1,"repositories_folder":"~/repos","agent_limit":8}` gives `<home>/repos` and 8. Text that is not JSON, `"version":2`, the unknown field `"agentLimit"`, the folder `repos`, and the agent limits 0 and 65 each give `ErrSettingsInvalid`. |

- **Acceptance:** `go test ./src/client/cmd/... ./src/client/internal/config/...` passes, and `go run ./src/client/cmd/lgtm-client version` prints `0.0.0-dev`.

**Item 2. The state folder and safe writes**

- **Files:** `src/client/internal/state/folder.go`, `atomic.go`, `testhook.go`, `folder_test.go`, `atomic_test.go` and `crash_test.go`, all in that folder.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 4 and 5.
- **Complexity:** medium, because the method of writing is known and fits in two small files, and each check has its own test.
- **Design statements:** D19, R9, the section "Joining" step 3 (a file only its user can read), and the sections "Commands go down" and "Events come up", whose store lives in this folder.
- **Risky:** yes, for ordering. Each mutation must make the named test fail: Writing the final file in place makes `TestWriteAtomicCrashAtEachStep` fail. Removing the sync of the temporary file makes `TestWriteAtomicSyncsBeforeRename` fail. Removing the check of group and other bits in `Open` makes `TestOpenRefusesExposedFolder` fail. Removing the `flock` makes `TestOpenRefusesSecondHolder` fail. Using a rename in `CreateExclusive` makes `TestCreateExclusiveNeverReplaces` fail. Syncing only the parent of the state folder makes `TestOpenSyncsWholeChain` fail. Returning a plain error when the folder sync fails after placement makes `TestWriteReportsPlacedNotSynced` fail.
- **Public names:**
  - Constants `LockFile = "lock"`, `SettingsFile = "settings.json"`, `KeyFile = "machine.key"`, `IdentityFile = "identity.json"`, `StoreFile = "client.db"`, `RepositoriesDir = "repositories"` and `TempPrefix = ".tmp-"`, and `type Paths struct { Dir, Lock, Settings, Key, Identity, Store, Repositories string }` with `func PathsIn(dir string) Paths`.
  - `type Folder struct` with unexported fields, `func Open(dir string) (*Folder, error)`, `func (f *Folder) Paths() Paths`, which returns a copy, and `func (f *Folder) Close() error`.
  - `func (f *Folder) Claim(name string) (Paths, func(), error)`, which returns the paths of the locked folder and a function that releases the claim. A second claim of the same name before its release returns `ErrStoreOpen`. A claim on a `nil` or zero folder, or after `Close`, returns `ErrFolderClosed`. `Close` returns `ErrStoreOpen` while a claim is held.
  - `func WriteAtomic(dir, name string, data []byte) error`, `func CreateExclusive(dir, name string, data []byte) error`, which returns an error that wraps `os.ErrExist` when the name exists, `func RemoveTemps(dir string) error` and `func SyncDir(dir string) error`.
  - `ErrNotAFolder`, `ErrFolderExposed`, `ErrFolderNotOwned`, `ErrFolderLocked`, `ErrFolderClosed`, `ErrParentUnsafe`, `ErrBadName`, `ErrStoreOpen` and `ErrPlacedNotSynced`. Writer: item 2. Readers: items 3 and 6, and plan `go-s3-client-link`.
  - For tests: the unexported `crashHook func(step string)`, `syncFile` and `syncFolder`, and in `testhook.go` the exported `SetCrashHookForTest` and `SetSyncFolderForTest`, which return a function that puts the old value back. The steps of a write are `temp-created`, `temp-written`, `temp-synced`, `placed` and `dir-synced`.
- **Behaviour:**
  - `Open` resolves the existing part of the path with `filepath.EvalSymlinks`. Every folder of it must belong to the user or to the superuser and have no write bit for group or others, except a folder of the superuser with the sticky bit. Otherwise `Open` returns `ErrParentUnsafe` and creates nothing. `Open` creates each missing folder with mode `0700`. It refuses a state folder that is a link or not a folder (`ErrNotAFolder`), has any bit of group or others (`ErrFolderExposed`), or belongs to another user (`ErrFolderNotOwned`), and it never changes the mode of an existing folder.
  - `Open` opens `lock` with `O_RDWR|O_CREATE|O_NOFOLLOW` and mode `0600`, and takes `flock` with `LOCK_EX|LOCK_NB`. Another holder gives `ErrFolderLocked`. Under the lock it calls `RemoveTemps`. Before it returns, `Open` syncs every folder of the resolved chain, from the root down to the state folder, so that a later `Open` makes durable what an earlier one created. A failed sync releases the lock and returns the error.
  - `WriteAtomic` and `CreateExclusive` refuse with `ErrBadName` a name that is empty, `.` or `..`, holds a separator, or starts with `TempPrefix`. They write `.tmp-<16 hexadecimal characters>` with `O_CREATE|O_EXCL` and mode `0600`, sync it, and then rename it over the name, or for `CreateExclusive` link it to the name with `os.Link` and remove the temporary name. They then sync the folder. A failure before `placed` changes nothing. A failed folder sync after `placed` returns an error that wraps `ErrPlacedNotSynced`: the new file is in place, and it may not survive a loss of power.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `folder_test.go` | `TestOpenCreatesPrivateFolders` | `Open` of a new path in `t.TempDir()` creates the folder with mode exactly `0700` and `lock` with mode exactly `0600`. A second `Open` removes `.tmp-abc` and `.tmp-abc.db` and keeps `keep.json`. `PathsIn("/s")` names `/s/lock`, `/s/settings.json`, `/s/machine.key`, `/s/identity.json`, `/s/client.db` and `/s/repositories`. |
| `folder_test.go` | `TestOpenRefusesExposedFolder` | An existing folder of mode `0750`, then `0705`, gives `ErrFolderExposed`, and its mode is unchanged. |
| `folder_test.go` | `TestOpenRefusesSymlinkAndUnsafeParent` | A link to a folder of mode `0700` gives `ErrNotAFolder`. A parent of mode `0777` without the sticky bit gives `ErrParentUnsafe`, and nothing is created in it. |
| `folder_test.go` | `TestOpenSyncsWholeChain` | With `syncFolder` recording, `Open` of `<tmp>/a/b/state` records a sync of every folder from the root to the state folder. When the first `Open` fails on the sync of `<tmp>/a`, it holds no lock, and a second `Open` succeeds and records `<tmp>`, `<tmp>/a`, `<tmp>/a/b` and the state folder. |
| `folder_test.go` | `TestOpenRefusesSecondHolder` | A second `Open` of the same folder, in the same process and in a child, gives `ErrFolderLocked`. After `Close`, `Open` succeeds. After a child that holds the folder is killed with `SIGKILL`, `Open` succeeds. |
| `folder_test.go` | `TestClaimAllowsOneStoreEach` | A second `Claim("store")` before the release gives `ErrStoreOpen`, and after it succeeds. `Close` while a claim is held gives `ErrStoreOpen`. A claim on a zero `Folder` and after `Close` gives `ErrFolderClosed`. Changing the copy that `Paths()` returned does not change the paths of a claim. |
| `atomic_test.go` | `TestWriteAtomicWritesPrivateFile` | The file holds exactly the data with mode exactly `0600`, writing `A` and then `B` leaves `B`, and no `.tmp-` name is left. `""`, `"."`, `".."`, `"a/b"` and `".tmp-x"` give `ErrBadName`. |
| `atomic_test.go` | `TestWriteAtomicSyncsBeforeRename` | With the hooks recording, the order is `temp-created`, `temp-written`, the file sync, `temp-synced`, `placed`, the folder sync and `dir-synced`, for both writes. |
| `atomic_test.go` | `TestWriteReportsPlacedNotSynced` | With `syncFolder` failing, `WriteAtomic` of `B` over `A` and `CreateExclusive` of `k` return errors for which `errors.Is(err, ErrPlacedNotSynced)` is true, and the files hold the new data. With `syncFile` failing, the error is not `ErrPlacedNotSynced`, the file holds `A`, and no `.tmp-` name is left. |
| `atomic_test.go` | `TestCreateExclusiveNeverReplaces` | A second `CreateExclusive` of `k` returns an error that wraps `os.ErrExist`, and `k` still holds exactly the first data. |
| `crash_test.go` | `TestWriteAtomicCrashAtEachStep` | For each step, a child writes `x` as `A` and then as 256 kibibytes of `B`, and exits with 86 at that step. For the first three steps `x` holds exactly `A`, and for `placed` and `dir-synced` exactly `B`. `CreateExclusive` of `k` leaves `k` missing for the first three steps and whole for the last two. |

The child processes run the helper `TestHelperStateProcess` in `crash_test.go`.
A kill leaves the operating system running, so these tests prove the order of the steps, and a post-build check covers a loss of power.

- **Acceptance:** `go test -race ./src/client/internal/state/...` passes, and the break reviewer's report names the seven mutations and the test that went red for each.

**Tasks of WS-1:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the commands of the program | `src/client/cmd/lgtm-client/main.go`, `run` (target). Pattern: `packages/cli/src/bin.ts`. |
| 1.2 | Write the cross build test | `src/client/cmd/lgtm-client/build_test.go`, `TestBuildsForLinuxAndMacOS` (target). Pattern: `deploy/test/checks.sh`, function `machines`, which builds the same package with `go build -o`. |
| 1.3 | Write the finder of the state folder and the settings reader | `src/client/internal/config/config.go`, `StateDir` and `LoadSettings` (target). Pattern: `floorplan/plan.json`, entry "Machine settings". |
| 2.1 | Write the checks, the lock and the claims of the folder | `src/client/internal/state/folder.go`, `Open` and `Claim` (target). Pattern: `packages/engine/src/harnesses/programs.ts`, `recordManagedInstallation`. |
| 2.2 | Write the safe writes and their test hooks | `src/client/internal/state/atomic.go`, `WriteAtomic` and `CreateExclusive`, and `testhook.go` (target). Pattern: `packages/engine/src/workflows/store.ts`, class `WorkflowStore`, whose save links a new file and renames over an old one. |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The program answers its three commands and builds for Linux and macOS. | `go test -run 'TestVersionPrintsVersion|TestJoinAndRunAreNotBuiltYet|TestUnknownCommandPrintsUsage|TestBuildsForLinuxAndMacOS' ./src/client/cmd/lgtm-client/` passes, and phase 5 runs it again on the merged work of all six items. |
| 2 | The state folder follows the one rule of the layout. | `go test -run 'TestStateDir' ./src/client/internal/config/` passes. |
| 3 | The state folder is created private, a folder that others can read or change is refused, and one client holds it at a time. | `go test -run 'TestOpenCreatesPrivateFolders|TestOpenRefusesExposedFolder|TestOpenRefusesSymlinkAndUnsafeParent|TestOpenRefusesSecondHolder|TestClaimAllowsOneStoreEach' ./src/client/internal/state/` passes. |
| 4 | A crash in a write leaves the old file or the new one, and an uncertain placement is reported. | `go test -run 'TestWriteAtomicCrashAtEachStep|TestWriteAtomicSyncsBeforeRename|TestWriteReportsPlacedNotSynced|TestOpenSyncsWholeChain' ./src/client/internal/state/` passes. |

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A crash leaves a half-written file that a later reader trusts. | high | The temporary file and the rename, proven by `TestWriteAtomicCrashAtEachStep`. |
| A loss of power undoes a rename or a new folder that was not synced. | high | Every folder of the chain is synced on each open, and the folder after each write, proven by `TestOpenSyncsWholeChain` and `TestWriteAtomicSyncsBeforeRename`. A real loss of power is a post-build check. |

**Verify:** the tests of items 1 and 2 under `go test -race ./src/client/...`. Device check: none for Linux. Phase 5 runs the suite on a Mac when one is available, because a folder sync there uses `F_FULLFSYNC`, and logs the gap when none is. Rollout: steps 1 and 2 of the rollout order.

**WS-2: The machine has a key that never leaves it**

This workstream makes the key that later proves the machine to the server, by D7, and refuses every way it could be read by someone else.

**Item 3. The key of the machine**

- **Files:** `src/client/internal/enroll/credential.go` and `credential_test.go`.
- **Needs first:** item 2, for `state.CreateExclusive`. Runs at the same time as item 6.
- **Complexity:** medium, because the standard library does the cryptography, and the work is file handling with one test for each refusal.
- **Design statements:** D7, the section "Joining" step 3, R9, and C10.
- **Risky:** yes, for keys. Each mutation must make the named test fail: Removing the check of group and other bits in `Load` makes `TestLoadRefusesExposedKey` fail. Writing the key with mode `0644` makes `TestCreateWritesPrivateKeyFile` fail. Opening without `O_NOFOLLOW` makes `TestLoadRefusesNonRegular` fail. Removing `String`, `GoString` or `Format`, or keeping the key in a field that is not behind a pointer, makes `TestCredentialNeverPrintsKey` fail. Replacing `state.CreateExclusive` with `state.WriteAtomic` makes `TestCreateRefusesExistingKey` fail.
- **Public names:** `type Credential struct`, whose only field is an unexported pointer to the key; `func Create(path string) (*Credential, error)` and `func Load(path string) (*Credential, error)`; the methods on the value `PublicKey() ed25519.PublicKey`, which returns a copy, `Sign(message []byte) []byte`, `Signer() crypto.Signer`, whose only methods are `Public` and `Sign`, `Fingerprint() string`, and `String`, `GoString` and `Format`, which each write `machine key ` and the fingerprint; and `ErrKeyExists`, `ErrKeyMissing`, `ErrKeyExposed`, `ErrKeyNotOwned`, `ErrKeyNotRegular`, `ErrKeyInvalid` and `ErrKeyUnreadable`. Writer: item 3. Reader: `join` and `run` of plan `go-s3-client-link`.
- **Behaviour:**
  - `Create` makes the key with `ed25519.GenerateKey(crypto/rand.Reader)`, encodes it with `x509.MarshalPKCS8PrivateKey` into one PEM block, and writes it with `state.CreateExclusive`. An existing file gives `ErrKeyExists` and is not changed. On `ErrPlacedNotSynced`, `Create` returns that error and no credential, and the caller follows rule 8 of the fold "What plan go-s3-client-link must do". `Load` opens the file with `O_RDONLY|O_NOFOLLOW|O_NONBLOCK`, so that a named pipe cannot block it, and checks the open file with `Stat` before it reads. A missing file, and only that, gives `ErrKeyMissing`. A link or a file that is not regular gives `ErrKeyNotRegular`. Any bit of group or others gives `ErrKeyExposed`. Another owner gives `ErrKeyNotOwned`. Any other failure gives `ErrKeyUnreadable`, which wraps the cause.
  - `Load` reads at most 4,097 bytes. The content must be exactly one PEM block of type `PRIVATE KEY` holding an Ed25519 key, with at most one newline after it. Anything else, or more than 4,096 bytes, gives `ErrKeyInvalid`. `Load` never changes the file, and no error message holds bytes of the key.
  - `Fingerprint` is `SHA256:` and the unpadded standard base64 of the SHA-256 (Secure Hash Algorithm, 256 bits) hash of the 32-byte public key.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `credential_test.go` | `TestCreateWritesPrivateKeyFile` | After `Create(path)`, the mode is exactly `0600`, the file holds one PEM block of type `PRIVATE KEY`, and `x509.ParsePKCS8PrivateKey` of it gives a key whose public half equals `PublicKey()`. |
| `credential_test.go` | `TestLoadReturnsSameKey` | `Load` after `Create` gives the same `PublicKey()` and `Fingerprint()`, and `Fingerprint()` is `SHA256:` and the 43 characters of `sha256.Sum256(PublicKey())` in unpadded standard base64. `ed25519.Verify(c.PublicKey(), m, c.Sign(m))` is true, a signature from `Signer().Sign(rand.Reader, m, crypto.Hash(0))` also verifies, and a type assertion of `Signer()` to `ed25519.PrivateKey` fails. |
| `credential_test.go` | `TestCreateRefusesExistingKey` | A second `Create` on the same path gives `ErrKeyExists`, and the bytes are unchanged. |
| `credential_test.go` | `TestLoadRefusesExposedKey` | After `os.Chmod` to `0640`, then `0604`, then `0660`, `Load` gives `ErrKeyExposed`. |
| `credential_test.go` | `TestLoadRefusesNonRegular` | A link to a valid key of mode `0600`, a folder, and a named pipe of mode `0600` with no writer each give `ErrKeyNotRegular`, the pipe within 1 second. |
| `credential_test.go` | `TestLoadRefusesBadContent` | A missing path gives `ErrKeyMissing`. A mode `0200` file gives `ErrKeyUnreadable` that wraps `fs.ErrPermission`, skipped as the superuser. `hello`, a `PUBLIC KEY` block, an RSA key, a valid key with `junk` before it, a second block or `x` after it, and 5,000 spaces after it each give `ErrKeyInvalid`. A valid key with one newline after it loads. After every refusal, the bytes and mode are unchanged. |
| `credential_test.go` | `TestCredentialNeverPrintsKey` | For the value, a pointer to it, and structs with an exported and an unexported field of type `Credential`, the verbs `%v`, `%+v`, `%#v`, `%s`, `%x` and `%q`, `json.Marshal` of the structs, and the text of every error above hold the 32-byte seed in no form: base64, hexadecimal, the decimal list of a byte slice, or raw bytes. `String()` starts with `machine key SHA256:`. |

A test cannot make a file owned by another user without the superuser, so the check of the owner is read by the reviewer and run as a post-build check.

- **Acceptance:** `go test -race ./src/client/internal/enroll/...` passes, and the break reviewer's report names the five mutations and the test that went red for each.

**Tasks of WS-2:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 3.1 | Write the making and loading of the key | `src/client/internal/enroll/credential.go`, `Create` and `Load` (target). Pattern: the design's section "Joining", step 3. |
| 3.2 | Write the signer that hides the key, and the printing methods | `src/client/internal/enroll/credential.go`, `Signer`, `String` and `Format` (target). Pattern: the interface `crypto.Signer` of Go's standard library. |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The key file is private, it is never replaced, and its signatures verify. | `go test -run 'TestCreateWritesPrivateKeyFile|TestLoadReturnsSameKey|TestCreateRefusesExistingKey' ./src/client/internal/enroll/` passes. |
| 2 | A key that others could read, that is not a plain file, or that holds anything else is refused, and the file is left alone. | `go test -run 'TestLoadRefusesExposedKey|TestLoadRefusesNonRegular|TestLoadRefusesBadContent' ./src/client/internal/enroll/` passes. |
| 3 | The key never reaches printed text. | `go test -run TestCredentialNeverPrintsKey ./src/client/internal/enroll/` passes. |

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The key reaches a log through a formatted print of the credential or of a struct that holds it. | high | The printing methods return only the fingerprint, and the key sits behind a pointer, proven by `TestCredentialNeverPrintsKey`. |
| A link or a named pipe at the key's path leaks the key or hangs the client. | high | `O_NOFOLLOW`, `O_NONBLOCK` and the check of the open file, proven by `TestLoadRefusesNonRegular`. |
| An agent of the same user reads the key file. | medium | Accepted by D19, which makes one user on a machine a convenience of wave 1. |

**Verify:** the tests of item 3. Device check: none, and the check of the owner runs as a post-build check. Rollout: step 3 of the rollout order.

**WS-3: The client can describe its machine and its harnesses**

Item 4 reads the machine, and item 5 finds the harnesses, for the facts, harnesses found and heartbeat of the design's table "What travels in wave 1". Neither sends anything. The types follow the fields of message `Client` in `src/contracts/proto/lgtm/v1/clients.proto`, so that the website shows the same meaning.

**Item 4. The facts and the load of the machine**

- **Files:** `src/client/internal/sysinfo/sysinfo.go`, `parse.go`, `sysinfo_linux.go`, `sysinfo_darwin.go`, `sysinfo_other.go`, `parse_test.go`, `sysinfo_linux_test.go`, `sysinfo_darwin_test.go`, `testdata/meminfo` and `testdata/loadavg`, all in that folder.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 2 and 5.
- **Complexity:** medium, because it spans two operating systems and a binary decoder, and every parser is a pure function with fixture tests.
- **Design statements:** the table "What travels in wave 1", the section "Knowing whether a machine is there", and C7.
- **Risky:** no.
- **Public names:** `const OSLinux = "Linux"` and `OSMacOS = "macOS"`; `type Facts struct { Hostname string; OS string; Arch string; Cores uint32; MemoryBytes uint64; ClientVersion string }`; `type Load struct { CPUPercent uint32; MemoryPercent uint32 }`; `func ReadFacts(clientVersion string) (Facts, error)` and `func ReadLoad() (Load, error)`. For the tests, `parse.go` has no build line and holds `parseMeminfo`, `parseLoadavg`, `decodeDarwinLoadavg`, `decodeDarwinUint64`, `cpuPercent(load1 float64, cores int) uint32` and `memoryPercent(total, available uint64) uint32`, and `sysinfo_linux.go` has `var procDir = "/proc"`.
- **Behaviour:**
  - `Hostname` comes from `os.Hostname`, `Arch` is `runtime.GOARCH`, and `Cores` is `runtime.NumCPU()`. On Linux, memory comes from `MemTotal` and `MemAvailable` of `<procDir>/meminfo`, in kibibytes times 1,024, and the load from the first field of `<procDir>/loadavg`.
  - On macOS, memory comes from `syscall.Sysctl("hw.memsize")`, the load from `syscall.Sysctl("vm.loadavg")`, and the memory in use from `100 - syscall.SysctlUint32("kern.memorystatus_level")`. `syscall.Sysctl` drops one trailing zero byte, so the decoders pad to 8 and to 24 bytes before they decode little-endian. `vm.loadavg` is three 32-bit values, 4 bytes of padding and a 64-bit scale, and a scale of 0 is an error. `cpuPercent` is `round(load1 / cores × 100)`, from 0 to 100. `memoryPercent` is `round((total − available) × 100 / total)`, and 0 when `total` is 0.
  - `sysinfo_other.go` has the build line `//go:build !linux && !darwin` and returns `errors.ErrUnsupported`.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `parse_test.go` | `TestParseLinuxFiles` | `MemTotal: 16384000 kB` and `MemAvailable: 4096000 kB` give 16,777,216,000 and 4,194,304,000 bytes, and a fixture without `MemAvailable` gives an error. `0.52 0.58 0.59 1/1234 5678\n` gives 0.52, and empty text gives an error. |
| `parse_test.go` | `TestDecodeDarwinValues` | 24 bytes holding 3072, 1024, 512, 4 zero bytes and the scale 2048 give a load of 1.5, also with the last zero byte removed, and a scale of 0 or 10 bytes give errors. The 8 little-endian bytes of 17,179,869,184 give that number, also with the last zero byte removed, and 3 bytes give an error. |
| `parse_test.go` | `TestPercentages` | A load of 2.0 on 4 cores gives 50, 9.0 on 4 gives 100, and 0 cores give 0. A total of 100 with 25 available gives 75, and a total of 0 gives 0. |
| `sysinfo_linux_test.go` | `TestReadOnLinux` | With `procDir` set to `testdata/`, `OS` is `Linux`, `MemoryBytes` is the fixture's total, `Cores` is `runtime.NumCPU()`, `ClientVersion` is the argument, `Hostname` is `os.Hostname()`, and the load is `cpuPercent(2.0, NumCPU)` and 75. |
| `sysinfo_darwin_test.go` | `TestReadLoadOnMacMatchesSystem` | Runs only on macOS. `MemoryBytes` equals `/usr/sbin/sysctl -n hw.memsize`. `CPUPercent` is within 2 of `cpuPercent` of the first value of `vm.loadavg` and `runtime.NumCPU()`. `MemoryPercent` is within 2 of 100 minus `kern.memorystatus_level`. |

- **Acceptance:** `go test ./src/client/internal/sysinfo/...` passes on Linux, and `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./src/client/internal/sysinfo/...` exits 0. On a Mac, `TestReadLoadOnMacMatchesSystem` passes in phase 5 of sprint 1, or of sprint 3 when no Mac is available in sprint 1.

**Item 5. The harness scan**

The scan does what the design says and no more, by the orchestrator's ruling 5 of 2026-11-26.
The client and the harnesses run as the same user in wave 1, so the rules of rounds 1 and 2 about who may write a folder or a helper protected nothing, and they are removed.

- **Files:** `src/client/internal/harness/registry.go`, `scan.go`, `registry_test.go` and `scan_test.go`, all in that folder.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 2 and 4.
- **Complexity:** medium, because it starts child processes under a time limit, a known pattern with clear tests.
- **Design statements:** the table "What travels in wave 1" (harnesses found), the section "The client in wave 1" (the four harnesses), D8, R9 and C7.
- **Risky:** no. It touches no sign-in, key, token or ordering.
- **Public names:**
  - `type Name string` with `Claude Name = "claude"`, `Codex = "codex"`, `Gemini = "gemini"` and `Cursor = "cursor"`, the values of `InstalledHarness.harness` in the contract.
  - `type Definition struct { Name Name; Programs []string; Version *regexp.Regexp }` and `func Definitions() []Definition`, in the order Claude, Codex, Gemini, Cursor.
  - `type Problem string` with `ProblemNone = ""`, `ProblemNotFound = "not found"`, `ProblemTimedOut = "timed out"`, `ProblemFailed = "failed to run"` and `ProblemNoVersion = "no version in output"`.
  - `type Found struct { Harness Name; Installed bool; Version string; Path string; Problem Problem }`. `Path` and `Problem` stay on the machine, and plan `go-s3-client-link` sends only `Harness` and `Version`.
  - `type ScanOptions struct { SearchPath []string; Home string; Timeout time.Duration }`, `DefaultTimeout = 3 * time.Second`, `WaitDelay = time.Second`, `func DefaultScanOptions(getenv func(string) string) ScanOptions`, which splits `PATH` and reads `HOME`, and `func Scan(ctx context.Context, opts ScanOptions) []Found`.
- **Behaviour:**
  - The programs are `claude`, `codex`, `gemini`, and `cursor-agent` then `agent` for Cursor. `Scan` drops empty and relative entries and repeats from `SearchPath`. For each program name, and each folder of that list in order, the folder joined with the name is a candidate when it exists.
  - A candidate runs with `exec.CommandContext`, its path and the one argument `--version`, never through a shell. Its environment is exactly `PATH=<the cleaned list>`, `HOME=<Home>` and `LANG=C`, its folder is `Home`, and its input is empty. At most 4 kibibytes of its output are read, its error output is thrown away, and `cmd.WaitDelay` is `WaitDelay`. All candidates of one harness share one deadline of `Timeout`, where 0 means `DefaultTimeout`. The four harnesses are scanned at the same time, so `Scan` ends within `Timeout` plus `WaitDelay` plus the time to clean up.
  - A candidate counts as installed only when it exits with 0 and its whole first line matches its harness's pattern. An exit of 0 with `exec.ErrWaitDelay`, from a grandchild that holds the output, counts as 0. The patterns: Claude `^(\d+\.\d+\.\d+) \(Claude Code\)$`, Codex `^codex-cli (\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)$`, Gemini `^(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)$`, and Cursor `^(\d{4}\.\d{1,2}\.\d{1,2}-[0-9a-f]{6,12})$`. `Version` is the first group.
  - The first candidate that counts gives `Installed: true`, its `Path` and its `Version`. When none counts, `Problem` is `ProblemNotFound` when no candidate existed, and otherwise the problem of the first candidate, with `ProblemFailed` for an exit other than 0 even when its line matched.
- **How the tests fake the programs:** each test writes scripts that start with `#!/bin/sh`, with mode `0755`, into folders under `t.TempDir()`, and passes only those folders as `SearchPath`. The scan never reads the real `PATH`. A fake that needs `sleep` names it by an absolute path that the test finds once with `exec.LookPath`. A fake that blocks first writes `<fake>.started`, and the test asserts that this file exists.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `scan_test.go` | `TestScanFindsFakePrograms` | Fakes print `2.1.325 (Claude Code)`, `codex-cli 0.98.0`, `0.68.0` and, as `cursor-agent`, `2026.11.20-abc1234`. All four are installed with those versions, and each `Path` is the fake's path. `Definitions()` gives `claude`, `codex`, `gemini` and `cursor` in that order, with Cursor's programs `cursor-agent` then `agent`. |
| `scan_test.go` | `TestScanEmptySearchPathFindsNothing` | With `SearchPath` empty, all four are not installed with `ProblemNotFound`, even when the machine has a real harness on its `PATH`. |
| `scan_test.go` | `TestScanRefusesExtraText` | A fake `claude` prints `2.1.325 (Claude Code) token=sk-planted-secret`, and a fake `codex` prints `warning: key 1.2.3-sk-planted` before `codex-cli 0.98.0`. Both give `ProblemNoVersion`, and no field of the result holds `sk-planted`. |
| `scan_test.go` | `TestScanGivesOnlyVersionAndCleanEnvironment` | With `t.Setenv("LGTM_PLANTED_SECRET", "planted")` and `SearchPath` holding `""`, `rel` and a test folder twice, the fake writes its arguments and environment to a file. The arguments are exactly `--version`, no line holds `planted`, `PATH` is that test folder alone, and `HOME` is the home. |
| `scan_test.go` | `TestScanNeverUsesShell` | A fake `claude` in a folder named `bin $(touch INJECTED);x` is installed, and no file `INJECTED` exists afterwards. |
| `scan_test.go` | `TestScanTimesOut` | A fake `gemini` writes `.started` and runs `sleep 30`. With `Timeout` 500 milliseconds, `Scan` returns within 3 seconds with `ProblemTimedOut`, and `.started` exists. With a context cancelled before the call, `Scan` returns within 2 seconds with none installed. |
| `scan_test.go` | `TestScanDoesNotWaitForGrandchild` | A fake `gemini` starts `sleep 30` in the background with the output inherited, prints `0.68.0` and exits 0. `Scan` returns within 3 seconds with Gemini installed, while that `sleep` is still alive. |
| `scan_test.go` | `TestScanSharesOneBudget` | With `Timeout` 500 milliseconds and fakes `cursor-agent` and two later `agent` fakes that sleep, Cursor gives `ProblemTimedOut` and no `agent` fake has a `.started` file. With `Timeout` 2 seconds, a `cursor-agent` that sleeps 1.5 seconds and exits 1, and a later `agent` that sleeps, the `agent` fake's `.started` file exists and `Scan` returns within 3.5 seconds. |
| `scan_test.go` | `TestScanReportsFailureAndNoVersion` | A `claude` that exits 1 and a `gemini` that prints `0.68.0` and exits 1 give `ProblemFailed`, and a `codex` that prints `hello` gives `ProblemNoVersion`. An `agent` that prints `1.4.2` gives `ProblemNoVersion`, and a later folder with an `agent` that prints `2026.11.20-abc1234` makes Cursor installed from there. |

- **Acceptance:** `go test -race ./src/client/internal/harness/...` passes, and the run starts no program outside `t.TempDir()` except the absolute `sleep`.

**Tasks of WS-3:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 4.1 | Write the pure parsers and decoders, and the fixtures | `src/client/internal/sysinfo/parse.go`, `parseMeminfo` and `decodeDarwinLoadavg` (target). Pattern: `src/contracts/proto/lgtm/v1/clients.proto`, message `Client`, fields `cpu_percent` and `memory_percent`. |
| 4.2 | Write the readers for Linux and macOS | `src/client/internal/sysinfo/sysinfo_linux.go` and `sysinfo_darwin.go`, `ReadFacts` and `ReadLoad` (target). Pattern: `parse.go` of task 4.1, and the field `MemTotal` of the Linux file `/proc/meminfo`. |
| 5.1 | Write the definitions of the four harnesses | `src/client/internal/harness/registry.go`, `Definitions` (target). Pattern: `packages/engine/src/harnesses/recipes.ts`, `installRecipes`. |
| 5.2 | Write the scan | `src/client/internal/harness/scan.go`, `Scan` (target). Pattern: `packages/engine/src/harnesses/programs.ts`, `makeHarnessPrograms`, inner function `inspect`. |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The facts and the load are read on Linux, and the values of macOS are decoded. | `go test ./src/client/internal/sysinfo/` passes, and `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./src/client/internal/sysinfo/` exits 0. |
| 2 | The scan finds fake harnesses, never touches the real ones, runs no shell, and leaks no secret. | `go test -run 'TestScanFindsFakePrograms|TestScanEmptySearchPathFindsNothing|TestScanRefusesExtraText|TestScanGivesOnlyVersionAndCleanEnvironment|TestScanNeverUsesShell' ./src/client/internal/harness/` passes. |
| 3 | A program that never answers does not stop the scan. | `go test -run 'TestScanTimesOut|TestScanDoesNotWaitForGrandchild|TestScanSharesOneBudget' ./src/client/internal/harness/` passes. |
| 4 | On a real Mac, the load and the memory match what the system reports. | On a Mac, `go test -run TestReadLoadOnMacMatchesSystem ./src/client/internal/sysinfo/` passes in phase 5. When no Mac is available, phase 5 logs the gap, and sprint 3 runs it again. |

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A harness prints a secret with its version, and the secret reaches the website. | high | Only a whole first line that matches the pattern counts, and the child gets no variable of the client, proven by `TestScanRefusesExtraText` and `TestScanGivesOnlyVersionAndCleanEnvironment`. |
| A program never ends, or a grandchild holds the output open. | medium | One deadline for all candidates of a harness, and `WaitDelay`, proven by `TestScanTimesOut`, `TestScanDoesNotWaitForGrandchild` and `TestScanSharesOneBudget`. |
| Another program called `agent` is taken for Cursor. | medium | Cursor's pattern needs a version that starts with a date, proven by `TestScanReportsFailureAndNoVersion`. |

**Verify:** the tests of items 4 and 5. Device check: done-when row 4 on a Mac, in phase 5. Rollout: step 2 of the rollout order.

**WS-4: Nothing the client must send or has done is lost in a crash**

This workstream builds the two records of the design's sections "Events come up" and "Commands go down", in one store file.
It offers them as an interface. It does not decide which events exist, when they are made, or how fresh they must be: that is plan `go-s3-client-link` and the contract, by the orchestrator's ruling 4 of 2026-11-26.
Restoring the state folder or the store to an earlier copy is outside wave 1, by ruling 1, so the store has no rule for that case.
Round 3 merged the outbox and the receipts into one item, because the open of the store and the receipts shared code (architect-r3 04, logic-r3 03).

**Item 6. The store: events and receipts**

- **Files:** `src/client/internal/link/store.go`, `outbox.go`, `receipts.go`, `store_test.go`, `outbox_test.go`, `receipts_test.go` and `store_crash_test.go`, all in that folder.
- **Needs first:** item 2. Its builder runs `go get go.etcd.io/bbolt@v1.4.3` in the same change as the first import, so that `scripts/check-go.sh` passes on that change, and the orchestrator compares the change of `go.mod` and `go.sum` with this line before it commits (UNREVIEWED FIX, architect-r4 09). Runs at the same time as item 3.
- **Complexity:** medium, because the store gives the atomic commit, and the item adds one creation step, one check at open and six small transactions, each with its own test.
- **Design statements:** the sections "Events come up" and "Commands go down", R6, R9 and C11.
- **Risky:** yes, for ordering. Each mutation must make the named test fail: Opening with `NoSync` set makes `TestOpenStoreCreatesSyncedPrivateFile` fail. Letting `Ack` accept a number at or above `Next()` makes `TestAckRefusesAhead` fail. Dropping the oldest event instead of refusing makes `TestAppendRefusesWhenFull` fail. Creating `client.db` in place makes `TestOpenStoreCrashDuringCreation` fail. Going on after a failed commit makes `TestCommitErrorFailsStore` fail, and checking the failed state outside the lock makes `TestFailureStopsWaitingWriter` fail. Skipping the walk of the event keys at open makes `TestOpenStoreRefusesDamagedFile` fail. Writing the `started` receipt after `Begin` returns makes `TestBeginWritesStartedBeforeReturn` and `TestReceiptsKillNeverRunsTwice` fail. Running a command whose `started` receipt came from an earlier process makes `TestStartedFromEarlierRunIsInterrupted` fail. Pruning a `started` receipt makes `TestFinishPrunesByAge` fail. Creating a store in `OpenStore` makes `TestOpenStoreMissingAfterJoin` fail. Opening an existing file for writing before the check makes `TestOpenStoreRefusesDamagedFile` fail.
- **Public names:**
  - `type StoreOptions struct { MaxEvents int; MaxBytes int64; MaxEventBytes int; MaxResultBytes int }`, with the defaults 10,000 events, 64 mebibytes, 1 mebibyte and 64 kibibytes. Round 4 removed `MaxReceipts` (UNREVIEWED FIX, ruling 1). A 0 means the default, and a negative value is refused.
  - `type Store struct`, `func CreateStore(f *state.Folder, opts StoreOptions) (*Store, error)`, which makes `client.db` when it is missing and otherwise opens it, and `func OpenStore(f *state.Folder, opts StoreOptions) (*Store, error)`, which opens an existing store only and returns `ErrStoreMissing` when there is none. Both claim `"store"` on the folder and use the path of the claim. Writer: item 6. Readers: `join`, which calls `CreateStore`, and `run`, which calls `OpenStore`, in plan `go-s3-client-link` (UNREVIEWED FIX, architect-r4 01).
  - Events: `type Event struct { Seq uint64; Payload []byte }`, `Append(payload []byte) (uint64, error)`, `Read(from uint64, limit int) ([]Event, error)`, `Ack(n uint64) error`, `Next() uint64`, `Len() int`, `Bytes() int64` and `Changed() <-chan struct{}`.
  - Receipts: `type Command struct { ID string; ExpiresAt time.Time }`, `type Decision int` with `DecisionRun = 1`, `DecisionRunning`, `DecisionReplay`, `DecisionInterrupted` and `DecisionExpired`, `Begin(c Command, now time.Time) (Decision, []byte, error)`, whose bytes are the stored result of a replay, and `Finish(id string, result []byte) error`.
  - `Close() error`, and `ErrOutboxFull`, `ErrEventTooLarge`, `ErrAckAhead`, `ErrInvalidCommand`, `ErrNotRunning`, `ErrResultTooLarge`, `ErrStoreFailed`, `ErrStoreDamaged`, `ErrStoreUnsafe`, `ErrStoreMissing`, `ErrSequenceExhausted` and `ErrStoreClosed`, beside `state.ErrStoreOpen`. Writer: item 6. Reader: plan `go-s3-client-link`.
  - For tests: the unexported `var commit = func(db *bolt.DB, fn func(*bolt.Tx) error) error { return db.Update(fn) }`, which every write goes through, `var wallClock = time.Now`, the machine's clock for pruning, and `var createHook func(step string)`, with the steps `temp-created`, `temp-initialized`, `placed` and `dir-synced`.
- **The format.** Bucket `meta` holds `version`, the text `1`, and `next`, an unsigned 64-bit big-endian number. Bucket `events` maps an event number, as an unsigned 64-bit big-endian key, to its payload. Bucket `receipts` maps a command's identifier to the JSON `{"version":1,"expires_at":"<RFC 3339 time in UTC>","state":"started"|"finished","result":"<base64>"}`, where `result` is left out while `started` (UNREVIEWED FIX, ruling 1). RFC 3339 is Request for Comments 3339, and UTC is Coordinated Universal Time. A receipt is valid when `version` is 1, `expires_at` is not zero, `state` is one of the two, and `result` is absent for `started` and at most `MaxResultBytes` for `finished`. Any other value is invalid (UNREVIEWED FIX, architect-r4 06). The count and the bytes of the events are not stored: `OpenStore` counts them, so they cannot disagree with the events (architect-r3 09).
- **Behaviour of `CreateStore` and `OpenStore`:**
  - When `client.db` does not exist, `CreateStore` opens `.tmp-<16 hexadecimal characters>.db` with mode `0600`, writes the three buckets, `version` and `next` = 1 in one transaction, closes it, links it to `client.db`, removes the temporary name, and syncs the folder. A stop before the link leaves only a temporary file, which `state.Open` removes. A link that finds `client.db` keeps that file. When `client.db` does not exist, `OpenStore` returns `ErrStoreMissing` and makes no file (UNREVIEWED FIX, architect-r4 01). Both then check `client.db` with `os.Lstat`. A link, a file that is not regular, a bit for group or others, or another owner gives `ErrStoreUnsafe`. A file of 0 bytes gives `ErrStoreDamaged` before the library opens it, because the library would write a new store into it (architect-r3 08, logic-r3 05).
  - They first open the file with `bolt.Open` and `ReadOnly` set, with a lock time limit of 1 second, so that the check cannot write. A time-out gives `state.ErrStoreOpen`, and any other failure gives `ErrStoreDamaged`. In one read transaction, the check covers the version, the three buckets and `next`, which must be 8 bytes and not 0, and walks every event key. Each key must be 8 bytes and not 0, each must be one above the one before, and when the queue is not empty the last must be exactly `next` minus 1 (UNREVIEWED FIX, architect-r4 04, logic-r4 02). Only after the check passes do they close that handle and open the file for writing, with mode `0600` and `NoSync` false, while the claim stays held (UNREVIEWED FIX, architect-r4 05, logic-r4 03). The walk gives the count and the bytes. Any failure gives `ErrStoreDamaged` (architect-r3 09, logic-r3 05). UNREVIEWED MECHANISM: the walk replaces the stored counts of round 2.
  - On any refusal, it closes the library's handle and releases the claim. It writes nothing to an existing file, and it never deletes, moves or rebuilds one. It then syncs the folder and returns.
- **Behaviour of the events:**
  - `Append` refuses a payload above `MaxEventBytes` with `ErrEventTooLarge`. In one transaction it refuses with `ErrOutboxFull` when the count would pass `MaxEvents` or the bytes would pass `MaxBytes`, stores the payload under `next`, and raises `next`. When `next` is `math.MaxUint64`, it returns `ErrSequenceExhausted` and changes nothing, so the number never wraps (UNREVIEWED FIX, logic-r4 02). It returns the number only after the commit returned, which syncs the file. It then signals `Changed()` without blocking. `Read(from, limit)` returns, in order, at most `limit` stored events whose number is at least `from`, with every payload copied.
  - `Ack(n)` removes every stored event up to `n` in one transaction, and does nothing when none is stored. When `n` is at least `Next()`, it returns `ErrAckAhead` and changes nothing: the store never handed that number out (ruling 2).
- **Behaviour of the receipts:**
  - `Begin` refuses with `ErrInvalidCommand` an identifier that is not a lowercase universally unique identifier (UUID) of 36 characters in canonical form, a zero `ExpiresAt`, and a zero `now`, and it writes nothing then. A `finished` receipt gives `DecisionReplay` and its result. A `started` receipt of a command that this store's process is running gives `DecisionRunning`. A `started` receipt from an earlier process, or an invalid value, gives `DecisionInterrupted` (UNREVIEWED FIX, architect-r4 06). With no receipt, `now` at or after `ExpiresAt` gives `DecisionExpired`. None of these writes anything.
  - Otherwise `Begin` writes a `started` receipt with the command's `ExpiresAt` in one transaction, marks the command as running in memory, and returns `DecisionRun` only after the commit returned. `Finish` returns `ErrNotRunning` for a command that this process is not running. It refuses a result above `MaxResultBytes` with `ErrResultTooLarge` and writes nothing, and the command stays running. Otherwise one transaction writes the receipt as `finished` with the result, and removes every valid `finished` receipt whose `expires_at` is more than 24 hours before `wallClock()`. A `started` receipt and an invalid value are never removed, and there is no limit of count (UNREVIEWED FIX, ruling 1 of round 4, architect-r4 03, logic-r4 01).
  - UNREVIEWED MECHANISM: a receipt has only the two written states of ruling 3. "Interrupted" is computed, not written: the process holds the folder's lock, so a `started` receipt that it is not running came from an earlier process. The stored state is the truth after any crash or failed commit: a `finished` receipt replays, and a `started` one is interrupted (architect-r3 11, logic-r3 07). The design removes receipts older than one day, and the contract has a machine answer every attempt until 24 hours after `expires_at`, so the age is measured from the stored expiry.
- **Behaviour of the whole store:** one lock guards every method that touches the file. Under it, a method checks for the closed and the failed state, runs its transaction, and on a failed commit puts the store into the failed state before it lets the lock go (logic-r3 04). In the failed state, every later call returns `ErrStoreFailed`, and the fold "What plan go-s3-client-link must do" says that `run` then exits. `Close` closes `Changed()`, the file and the claim, works in the failed state too, and returns nil the second time. After `Close`, every method returns `ErrStoreClosed`, and `Next`, `Len` and `Bytes` keep their last values.
- **Tests written first:** every test makes its store with `CreateStore`, and a reopen uses `OpenStore`.

| Test file | Test | What it asserts |
|---|---|---|
| `store_test.go` | `TestOpenStoreMissingAfterJoin` | On a folder with no `client.db`, `OpenStore` gives `ErrStoreMissing`, and the folder holds no new file. `CreateStore` on a folder with a valid store keeps it, and `Next()` is unchanged (UNREVIEWED FIX, architect-r4 01). |
| `store_test.go` | `TestOpenStoreCreatesSyncedPrivateFile` | On a new folder, `client.db` has mode exactly `0600`, `Next()` is 1, no `.tmp-` name is left, and the store's `NoSync` is false, read through an unexported accessor. With `state.SetSyncFolderForTest` recording, each `OpenStore` syncs the state folder, and with the sync failing it returns an error and no store. |
| `store_test.go` | `TestOpenStoreRefusesUnsafeFile` | A `client.db` that is a link to a valid store, and one of mode `0644`, give `ErrStoreUnsafe`. A second `OpenStore` on the same folder gives `ErrStoreOpen`, and after `Close` it succeeds. |
| `store_test.go` | `TestOpenStoreRefusesDamagedFile` | An existing empty file, 64 kibibytes of random bytes, a valid store cut in half, a store whose `version` is `2`, a store with the event keys 1 and 3, a store whose last key equals `next`, a store with the keys 3 and 4 and `next` 6, a store with the key 0, a store with `next` 0, and a store of `version` 2 written with the library's `NoFreelistSync` each give `ErrStoreDamaged` (UNREVIEWED FIX, architect-r4 04, architect-r4 05, logic-r4 02, logic-r4 03). After each, the file's bytes are unchanged, no other file was made, and a second `OpenStore` gives `ErrStoreDamaged` again and not `ErrStoreOpen`. |
| `store_test.go` | `TestOpenStoreWritesNothingToExistingFile` | A valid store with three events and a `started` receipt, opened, closed and opened again, has the same bytes as before the first open. |
| `store_test.go` | `TestCommitErrorFailsStore` | With `commit` committing and then returning an error, `Append(A)` returns an error that wraps `ErrStoreFailed`, the next `Append` returns `ErrStoreFailed`, and after a reopen `A` is stored and the next number follows it. With `commit` returning an error without committing, `A` is absent after the reopen. No number ever names two payloads. |
| `store_test.go` | `TestFailureStopsWaitingWriter` | The first `Append` is held inside `commit` until the test releases it with an error. A second `Append` starts and waits. After the release, the second returns `ErrStoreFailed`, and `commit` was entered only once (logic-r3 04). |
| `store_test.go` | `TestCloseKeepsSnapshot` | A goroutine waiting on `Changed()` returns within 1 second of `Close`. `Next`, `Len` and `Bytes` keep their values, every other method returns `ErrStoreClosed`, and a second `Close` returns nil. `Close` after a failed commit returns nil. |
| `outbox_test.go` | `TestAppendReadAndReopen` | Five appends return 1 to 5. After a reopen and `Ack(2)`, `Read(0, 10)` returns 3, 4 and 5 with their payloads, `Read(4, 10)` returns 4 and 5, `Read(0, 2)` returns 3 and 4, `Len()` is 3, and the next append returns 6. |
| `outbox_test.go` | `TestAckRemovesUpToNumber` | After five appends, `Ack(3)` leaves 4 and 5, `Ack(2)` and `Ack(3)` change nothing, `Ack(5)` leaves none, and `Ack(5)` again returns nil. After a reopen, the next append returns 6. |
| `outbox_test.go` | `TestAckRefusesAhead` | With events 1 to 3 stored, `Ack(4)` and `Ack(math.MaxUint64)` return `ErrAckAhead`, and events 1 to 3 stay stored. On a new store, `Ack(1)` returns `ErrAckAhead`. |
| `outbox_test.go` | `TestAppendRefusesWhenFull` | With `MaxEvents` 3, the fourth append returns `ErrOutboxFull` and events 1 to 3 are unchanged, and after `Ack(1)` an append returns 4. With `MaxBytes` 10, appends of 6 and then 5 bytes give success and then `ErrOutboxFull`. With `MaxEventBytes` 8, a payload of 9 bytes gives `ErrEventTooLarge`, and `Next()` is unchanged. A store whose `next` the test sets to `math.MaxUint64` gives `ErrSequenceExhausted` on the next append, and nothing changes (UNREVIEWED FIX, logic-r4 02). |
| `outbox_test.go` | `TestConcurrentAppendsAreUnique` | 8 goroutines append 50 events each. The numbers are 1 to 400 with no repeat, `Read(0, 1000)` returns 400 events in order, and each append signals `Changed()`. |
| `receipts_test.go` | `TestBeginRunsAndReplays` | A new valid command gives `DecisionRun`, and a second `Begin` before `Finish` gives `DecisionRunning`. After `Finish(id, "ok")`, `Begin` gives `DecisionReplay` and `ok`, also after a reopen. |
| `receipts_test.go` | `TestBeginWritesStartedBeforeReturn` | With `commit` committing and then waiting for the test, a `Begin` in a goroutine has not returned after 200 milliseconds, and after the release it returns `DecisionRun`. |
| `receipts_test.go` | `TestStartedFromEarlierRunIsInterrupted` | After `Begin` with no `Finish` and a reopen, `Begin` gives `DecisionInterrupted`, and `Finish` gives `ErrNotRunning`. A value `{`, and values with `version` 2, the state `done`, a zero `expires_at`, and a `started` state with a result, also give `DecisionInterrupted`, and a later `Finish` does not remove them (UNREVIEWED FIX, architect-r4 06). After two more reopens the answers are the same, and the file's bytes did not change. |
| `receipts_test.go` | `TestOnlyRunAndFinishWrite` | With `commit` counting, a replay, a running command, an interrupted one, an expired one and a refused identifier make no commit. With `MaxResultBytes` 4, `Finish` with 5 bytes gives `ErrResultTooLarge` and no commit, and `Finish` with 4 bytes then succeeds. |
| `receipts_test.go` | `TestBeginRefusesExpiredAndBadInput` | For two new identifiers, `now` one second before `ExpiresAt` gives `DecisionRun`, and `now` equal to it gives `DecisionExpired`. `../machine.key`, `a/b`, an uppercase UUID, a UUID without hyphens, the empty text, a zero `ExpiresAt` and a zero `now` give `ErrInvalidCommand`. |
| `receipts_test.go` | `TestFinishPrunesByAge` | With `wallClock` fixed at T, a finished receipt whose `expires_at` is T minus 25 hours is removed by the next `Finish`, one at T minus 23 hours is kept, and a `started` one at T minus 48 hours is kept. 2,000 finished receipts within the day are all kept (UNREVIEWED FIX, ruling 1). |
| `receipts_test.go` | `TestFinishCommitErrorKeepsStoredState` | With `commit` committing and then returning an error in `Finish`, after a reopen `Begin` gives `DecisionReplay` with the result. With `commit` returning an error without committing, after a reopen `Begin` gives `DecisionInterrupted` (architect-r3 11). |
| `receipts_test.go` | `TestConcurrentBeginRunsOnce` | 16 goroutines call `Begin` with the same command, and exactly one gets `DecisionRun`. |
| `store_crash_test.go` | `TestOpenStoreCrashDuringCreation` | For each step of creation, a child opens a new store and exits with 86 at that step. After `state.Open` and `OpenStore` in the parent, `Next()` is 1, and no `.tmp-` name is left. |
| `store_crash_test.go` | `TestStoreKillKeepsEveryNumberedEvent` | A child appends payloads `event-<n>` of 1 byte to 64 kibibytes, prints `appended <n>` after each return, and calls `Ack` at every tenth number and prints `acked <n>` after it returns. The parent kills it with `SIGKILL` at a random moment, 30 times from a logged seed, with at least 100 numbers printed in all. After each kill, `OpenStore` succeeds, each printed number is at most the highest printed `acked` number or is stored with its exact payload (UNREVIEWED FIX, architect-r4 04), the stored numbers have no gap, and the next child's first number is above every number printed before. |
| `store_crash_test.go` | `TestStoreKeepsOrderAndOnceAcrossRestart` | The parent plays the server, with a list of applied events. In 20 rounds, a child appends 1 to 20 events and is killed at a random moment. The parent opens the store, calls `Ack` with its last applied number, applies every event that is its last plus 1, and acknowledges. At the end, every printed number was applied exactly once, in rising order, with its exact payload. |
| `store_crash_test.go` | `TestReceiptsKillNeverRunsTwice` | A child calls `Begin` for identifiers from a counter. After `DecisionRun` it appends the identifier to the file `effects` with a sync, prints `begun <id>`, calls `Finish` with `result-<id>`, and prints `finished <id>`. The parent kills it at a random moment, 30 times from a logged seed. No identifier appears twice in `effects`, each `finished` one replays its exact result, and `Begin` of each one in `effects` never gives `DecisionRun`. |

The child processes run the helper `TestHelperStoreProcess` in `store_crash_test.go`, with a mode for each test.

- **Acceptance:** `go test -race ./src/client/internal/link/...` passes, and the break reviewer's report names the nine mutations and the test that went red for each.

**Tasks of WS-4:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 6.1 | Write the creation, the check at open, the lock and the failed state of the store | `src/client/internal/link/store.go`, `CreateStore`, `OpenStore` and `Close` (target). Pattern: `CreateExclusive` of task 2.2, which also links a finished file to its name, and `Open` in `db.go` of `go.etcd.io/bbolt`. |
| 6.2 | Write the events | `src/client/internal/link/outbox.go`, `Append`, `Read` and `Ack` (target). Pattern: `floorplan/plan.json`, entry "Events waiting to be sent". |
| 6.3 | Write the receipts | `src/client/internal/link/receipts.go`, `Begin` and `Finish` (target). Pattern: the design's section "Commands go down", and the comment at the top of `.plans/plans/go-s0-contract/draft/commands.proto`. |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Every event that got a number survives a kill, whole and in order, and arrives once across a restart. | `go test -run 'TestStoreKillKeepsEveryNumberedEvent|TestStoreKeepsOrderAndOnceAcrossRestart|TestOpenStoreCreatesSyncedPrivateFile' ./src/client/internal/link/` passes. |
| 2 | A store that stopped while it was made opens as new, and a store that does not open is refused and left untouched. | `go test -run 'TestOpenStoreMissingAfterJoin|TestOpenStoreCrashDuringCreation|TestOpenStoreRefusesDamagedFile|TestOpenStoreRefusesUnsafeFile|TestOpenStoreWritesNothingToExistingFile' ./src/client/internal/link/` passes. |
| 3 | A number is never given to two events, an acknowledgement never passes the last number handed out, and a failed commit stops every writer. | `go test -run 'TestAckRemovesUpToNumber|TestAckRefusesAhead|TestConcurrentAppendsAreUnique|TestCommitErrorFailsStore|TestFailureStopsWaitingWriter' ./src/client/internal/link/` passes. |
| 4 | A full outbox refuses and loses nothing. | `go test -run TestAppendRefusesWhenFull ./src/client/internal/link/` passes. |
| 5 | A command that arrives twice runs once, and no command runs again after a kill or a failed commit. | `go test -run 'TestBeginRunsAndReplays|TestConcurrentBeginRunsOnce|TestStartedFromEarlierRunIsInterrupted|TestFinishCommitErrorKeepsStoredState|TestReceiptsKillNeverRunsTwice' ./src/client/internal/link/` passes. |
| 6 | A command past its expiry is not run, a bad identifier is never stored, and a finished receipt is removed one day after its expiry. | `go test -run 'TestBeginRefusesExpiredAndBadInput|TestOnlyRunAndFinishWrite|TestFinishPrunesByAge' ./src/client/internal/link/` passes. |

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| An event is sent before it is on disk, and a crash gives its number to another event. | high | The number is taken and the event stored in one commit that syncs before `Append` returns, proven by `TestStoreKillKeepsEveryNumberedEvent`. |
| A commit fails after the library wrote its pages, and the client cannot tell what is stored. | high | The store stops under its lock, `run` exits, and the next start reads the disk, proven by `TestCommitErrorFailsStore` and `TestFailureStopsWaitingWriter`. A failing disk is a post-build check. |
| A command runs twice after a crash. | high | The `started` receipt is committed before the command runs, and a `started` receipt from an earlier process is interrupted, proven by `TestReceiptsKillNeverRunsTwice`. |
| The machine's clock is wrong, and a receipt is removed while the server may still ask for it. | low | Accepted as a limit of wave 1 by ruling 1 of round 4: the client trusts its own clock. |
| An enrolled machine loses `client.db`, and a new store forgets a command that ran. | high | Only `join` makes a store. `run` gets `ErrStoreMissing` and stops before it dials, proven by `TestOpenStoreMissingAfterJoin` (UNREVIEWED FIX, architect-r4 01). |
| A store restored from an older copy deletes or repeats events, or runs a command again. | medium | Outside wave 1 by ruling 1. A store with fewer numbers than the server has applied gets `ErrAckAhead` on the first acknowledgement, and the client stops. The result file asks for an issue. |

**Verify:** the tests of item 6 under `-race`. Device check: none. The kill tests run on Linux, and phase 5 runs them on a Mac when one is available. Rollout: step 3 of the rollout order.

</details>

<details><summary><b>What plan go-s3-client-link must do</b></summary>

The pieces of this plan are safe only when their caller keeps these rules.
Plan `go-s3-client-link` is that caller, so its planner reads this list before freezing the plan.
Which events exist, when they are made, and how fresh they must be is that plan's choice and the contract's, by ruling 4.

**The store.**

1. When `Welcome` arrives, the sender calls `Ack(Welcome.last_event_number)` before it sends any event. `ErrAckAhead` means that the server has numbers this store never handed out: the client stops the stream, logs one sentence that names `client.db`, and does not retry until a person acts (ruling 2).
2. The sender sends events in order from `Read`, and calls `Ack` with `EventAck.through` of the current stream. The store checks only that the number is below `Next()`, so the sender keeps in memory the highest number it has written on this connection, and ends the stream as in rule 1 when an acknowledgement is above it or `Ack` returns `ErrAckAhead` (UNREVIEWED FIX, architect-r4 02).
3. Any error that wraps `ErrStoreFailed` makes `run` log one sentence and exit with a status other than 0, so that the service starts it again. `ErrStoreDamaged`, `ErrStoreUnsafe` and `ErrSequenceExhausted` make `run` exit with one sentence that names `client.db`, until the owner rules on decision 48.
4. A producer whose event gets `ErrOutboxFull` decides itself whether to keep, retry or skip it.
5. A command runs only after `Begin` returns `DecisionRun`, with `now` from the count of the server's time that the contract defines. `DecisionRunning` sends nothing. The caller keeps the newest attempt that arrived while a command runs, and sends the result under it, because the design ignores a result for an older attempt. The bytes given to `Finish` hold no attempt: the caller adds the attempt when it sends, and on a replay it uses the attempt that just arrived (UNREVIEWED FIX, logic-r4 04).
6. The caller calls `Finish` for every command it ran before it exits for any reason it controls, and `Restart` calls `Finish` before it exits. A result that gets `ErrResultTooLarge` is replaced by a short failure result.
7. The caller sends `DecisionReplay` as the stored result, `DecisionInterrupted` as `COMMAND_OUTCOME_INTERRUPTED`, and `DecisionExpired` as `COMMAND_OUTCOME_EXPIRED`.

**Joining and the state folder.**

8. `join` opens the state folder with `state.Open`, refuses a folder that holds `identity.json`, and calls `enroll.Load` first. Only on `ErrKeyMissing` does it call `enroll.Create`. On `ErrPlacedNotSynced`, it calls `state.SyncDir`, and it calls the server only after that sync succeeds.
9. `join` calls `CreateStore` before it writes `identity.json`, and a retry of a join reuses that store (UNREVIEWED FIX, architect-r4 01). It writes `identity.json` with `state.WriteAtomic` last, only after the server confirmed the join, and reports success only after the folder sync succeeded.
10. A key without `identity.json` is a join that did not finish, and the next `join` reuses the key while the token is valid. After the token expired, decision 79 is for the owner.
11. `run` holds the state folder with `state.Open`, refuses to start without a valid `identity.json`, and logs the resolved state folder once. It opens the store with `OpenStore`, and on `ErrStoreMissing` it stops with one sentence before it dials and changes no file (UNREVIEWED FIX, architect-r4 01).

</details>

<details><summary><b>Post-build checks</b></summary>

These checks need the built code, so they run in phase 5 of the build loop, or in sprint 3 where a row says so.

| # | Check | Scenario | Expected result | From |
|---|---|---|---|---|
| 1 | A real loss of power | On a Linux test machine with the state folder on its own disk, and on a Mac when one is available, a loop appends events and runs commands, and the power is cut 10 times at random moments. | After each restart, `OpenStore` succeeds or names `client.db`. Every printed event number is stored or acknowledged, and no command appears twice in the file of effects. | rounds 1 and 2 |
| 2 | The owner checks as the superuser | A person runs the tests of items 2, 3 and 6 as the superuser, with a key file, a store file and a state folder owned by another user. | `Open` returns `ErrFolderNotOwned`, `Load` returns `ErrKeyNotOwned`, and `OpenStore` returns `ErrStoreUnsafe`. | round 1 |
| 3 | The real version lines | On a machine with each of the four harnesses installed, a person runs `Scan`. | Each installed harness is found with its version. A harness whose line does not match is recorded, and its pattern is fixed. | round 1 |
| 4 | A disk that fails a sync | On Linux, the state folder sits on a device-mapper target that is switched to return errors while the client appends events, and then switched back. | The client exits with one sentence about `ErrStoreFailed`. After the restart, no number names two payloads. | round 2 |
| 5 | The order after a reconnection | In sprint 3, the server is stopped while the client appends 50 events, and then started again. | The server applies all 50 once and in order. | round 1 |
| 6 | Events while the server is stopped | In sprint 3, check C11 of `deploy/test/checks.sh` runs. | The check passes with the events that plan `go-s3-client-link` chose to make. | round 2 |
| 7 | Receipts removed by age | In sprint 3, a command runs and finishes, its result is lost on the way, and the machine's clock is set 23 hours and then 25 hours past its expiry before the server's next attempt. | At 23 hours the attempt gets the recorded result. At 25 hours the receipt is gone, and the attempt gets `COMMAND_OUTCOME_EXPIRED` without running. | ruling 1 of round 4, architect-r4 03, logic-r4 01 |
| 8 | A lost store after a join | On a joined machine, `client.db` is removed while the server holds a pending command and has applied no event, and `lgtm-client run` starts. | `run` exits with one sentence about the missing store before it dials, and the state folder holds the same files as before. | architect-r4 01 |
| 9 | An acknowledgement above what was sent | In sprint 3, a test server sends `EventAck.through = 3` after the client wrote only event 1 of three. | The client ends the stream, and events 2 and 3 are still stored. | architect-r4 02 |
| 10 | The module file | The merge of item 6 is checked with `scripts/check-go.sh`. | The check exits 0, and `go.mod` names `go.etcd.io/bbolt v1.4.3` and nothing else new. | architect-r4 09 |
| 11 | The check at open on every allowed library | With `go.etcd.io/bbolt` v1.4.3, and v1.5.0 when the toolchain allows it, `TestOpenStoreRefusesDamagedFile` runs, including the store of `version` 2 written with `NoFreelistSync`. | Every fixture gives `ErrStoreDamaged`, and the bytes of each file are unchanged. | architect-r4 04, architect-r4 05, logic-r4 02, logic-r4 03 |
| 12 | Receipts that decode but are invalid | A person stores each invalid receipt of `TestStartedFromEarlierRunIsInterrupted` in a built store, and the server sends that command again. | Each answer is `COMMAND_OUTCOME_INTERRUPTED`, no command runs, and the values are still there after a later `Finish`. | architect-r4 06 |
| 13 | A new attempt while a command finishes | In sprint 3, the stream breaks while a scan runs, attempt 2 arrives on the new stream, and the test forces the finish before and after that arrival. | In both orders the server records the result once, under attempt 2. | logic-r4 04 |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe: (1) the six work items and (2) the four points of the brief of 2026-11-25; (3) the four points of the task file `.foundry/go-rewrite/tasks/plan-go-s1-client-local.md` under "Specific to this plan"; (4) the statements of the design listed in `scouting.md`; (5) point 4 of the epic's section "What the teams must agree on"; (6) the findings of rounds 1 to 4 in `attack/`; and (7) the orchestrator's rulings of 2026-11-26 in the task files of the folds of rounds 2, 3 and 4.

| Source item | Workstream | Status |
|---|---|---|
| Brief items 1 to 6: the program and its settings, the key, the facts, the scan, the events and the receipts | Items 1, 3, 4, 5 and 6, on top of item 2 | mapped |
| Brief points 1 and 2, a full buffer and its size | Item 6, `ErrOutboxFull` and `StoreOptions`; decisions 14 and 15 | mapped |
| Brief point 3, the load on macOS without a shell | Item 4; decision 23 | mapped |
| Brief point 4, receipts in their own file | Item 6, `receipts.go` in the one store; decisions 25 and 55 | mapped |
| Task points 1 to 4: the layout, the fake programs, a crash in a write, and only `go.mod` from `go-s0-contract` | The layout of the state folder; item 5's fakes; the crash tests of items 2 and 6; rollout step 1 and decision 57 | mapped |
| D2, R1 and R10: Go, one folder for each part, real tests | Every item, under `src/client/internal/`; the guardrails | mapped |
| D7, C10 and the section "Joining" step 3: the key pair that the server knows | WS-2, item 3 | mapped |
| D8, D16 and the section "The client in wave 1" | Item 1 has no command that serves. Item 5 reports the plain programs, and wave 2 starts their ACP forms. | mapped |
| D19, one user on a machine | Items 2 and 3, and item 5 by ruling 5 | mapped |
| R6, R9, C11 and the sections "Commands go down" and "Events come up" | WS-4, item 6, the client's half; R9 also in items 3 and 5 | mapped |
| C7 and the sections "What travels in wave 1" and "Knowing whether a machine is there" | WS-3, items 4 and 5 | mapped |
| Epic agreement point 4, the layout of the state folder | Implementation details, "The layout of the state folder" | mapped |
| Findings of rounds 1 to 4 | Fixed at the places each register names; kept by the caller in the fold "What plan go-s3-client-link must do"; checked in "Post-build checks"; or for the owner in decisions 48 and 79 | mapped |
| The rulings of rounds 2, 3 and 4 | Items 5 and 6, and the layout; decisions 55, 72 to 80, and 87 to 92 | mapped |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "What this project is": the server and the client are written from scratch in Go. | Every item is Go under `src/client/`. Nothing is ported from `packages/`, which is read only as a pattern. |
| The epic's table of teams: the Client team edits only `src/client/`. | Every file of every item is under `src/client/`. The cross build test sits in `src/client/cmd/lgtm-client/` because `scripts/` belongs to the Contract team. |
| `AGENTS.md`, "Writing style for documents", enforced by `scripts/check-private.mjs`. | The plan names no home folder of a real user, no network address and no quotation. Paths start with `~/` or are relative to the repository. Acronyms are expanded the first time. |
| The design's rule 9: secrets never reach a log. | Item 3 never prints the key. Item 5 keeps only the version text. |
| The design's rule 10: tests run against a real PostgreSQL. | This plan needs no database server. The store is an embedded file, and its tests open a real one in a temporary folder. The crash tests kill real processes and read real files. |
| The rule for planners: name no model and no harness for a seat. | The four harnesses appear only as data that item 5 looks for. Decision 33 records this. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch. This section names none, by the owner's rule of 2026-10-16.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. The program and its settings | backend | client/backend | low | A fixed set of commands and a small reader of one file, with no design choice left. | no | `go.mod` |
| 2. The state folder and safe writes | backend | client/backend | medium | A known method of writing in two small files, made risky by ordering and covered by named crash tests. | no | `go.mod` |
| 3. The key of the machine | backend | client/backend | medium | The standard library does the cryptography, and the work is file handling with one test for each refusal. | no | 2 |
| 4. The facts and the load | backend | client/backend | medium | Two operating systems and a binary decoder, all pure functions with fixtures. | no | `go.mod` |
| 5. The harness scan | backend | client/backend | medium | Child processes under a time limit, a known pattern with clear tests. Round 3 removed the trust rules. | no | `go.mod` |
| 6. The store: events and receipts | backend | client/backend | medium | The library gives the atomic commit. The item adds one creation step, one read-only check at open and six small transactions under one lock, each with its own test. | no | 2 |

No unit is rated high, and no unit changes an entry point that outsiders can reach, so none needs an attack pass. Every unit uses the scope file `.plans/scopes/client/backend/scope.json`.

Load: each unit has one test writer, one builder, one code reviewer and the reader of the third vendor, and the risky units 2, 3 and 6 add one break reviewer each. That is 6 test writers, 6 builders, 6 code reviewers, 6 readers and 3 break reviewers, 27 launches in all. With builders alternating between the two main vendors, one takes 11 launches and the other 10, and the third vendor takes the 6 reader seats. The orchestrator checks these counts against each vendor's weekly meter at the start of sprint 1. If one vendor would run out, the units of WS-3, which share nothing with the others, move later in the sprint first.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 0 | none yet | The planner stopped after the self-check. The orchestrator runs the review rounds with other vendors. |
| 1 | 57 findings from five reviewers: 18 blockers, 30 majors and 9 minors. | Outcomes in `attack/round-1-findings.md`. C11 was stuck, with 55 of 116 points. |
| 2 | 32 findings from three reviewers: 9 blockers, 18 majors and 5 minors. | Outcomes in `attack/round-2-findings.md`. C11 was stuck, with 52 of 87 points, and a ruling replaced the outbox of files by one store. |
| 3 | 24 findings from two reviewers: 7 blockers and 17 majors, in 13 groups. | Every finding has an outcome in `attack/round-3-findings.md`. C11 is stuck a third time, with 30 of 63 points. By eight rulings of the orchestrator, the plan was cut from about 1,000 lines to under 500: the trust rules of the scan, the sent mark, the clock and the third state of the receipts, the producer of events and the rules for a restored store are gone, and items 6 and 7 are one item. Two choices are for the owner. Round 4 attacks only blockers and majors. |
| 4 | 15 findings from two reviewers: 4 blockers and 11 majors, in 10 groups. | Every finding has an outcome in `attack/round-4-findings.md`. R6 is stuck with 24 of 48 points, and C11 with 15. By six rulings of the orchestrator, receipts are removed by age as the design says, only `join` makes the store, and the bound of an acknowledgement on a connection lives in the sender. Round 4 was the last round without the owner, so every fix is marked UNREVIEWED FIX and has a post-build check, rows 7 to 13. |

Known gaps from the self-check, for the reviewers:

- Gate 1: row 5 of `decisions.md` is open. It is the owner's decision on the cut of wave 1 into sprints, and the planner cannot close it. Decision 34 records that the plan builds on the cut as written.
- Gate 10: the remaining weekly allowance of each vendor is not known to the planner.
- The user-flow test sets `XDG_STATE_HOME`, which ruling 6 no longer reads. The result file asks the Stack team to set `LGTM_CLIENT_STATE_DIR` instead.
- The owner checks of items 2, 3 and 6 have no test, and the tests of macOS run only when a Mac is available in phase 5.

</details>
