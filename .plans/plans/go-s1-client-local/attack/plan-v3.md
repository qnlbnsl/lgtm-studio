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
It keeps two records on disk, in one store with transactions: the events that wait to be sent to the server, and a receipt of each command it ran.

The two records carry the promises of the design that no event is lost and that no command runs twice.
Both must survive a crash in the middle of a write.
Since round 2, both live in the embedded store `go.etcd.io/bbolt`, in one file of the state folder, by a ruling of the orchestrator (UNREVIEWED MECHANISM).
The store commits each change whole or not at all, and it syncs the disk before a commit returns.
Adding an event takes its number and stores it in one commit, so a number that was handed out and an event that is stored are the same fact.
When a commit fails, the client stops and starts again, instead of guessing what is on disk.
The key and the settings stay plain files, written in one safe way: write a temporary file, sync it, rename or link it to its name, and sync the folder.
Tests kill a child process at random moments and check after each kill that every numbered event is whole and that no command can run again.
A real loss of power is checked after the build, in the section "Post-build checks".

The plan also fixes the layout of the state folder.
Plan `go-s3-client-link` reads the same files later, so every name, format and permission is written down here.

No new owner decision is needed to start.
The cut of wave 1 into sprints is still open with the owner, and this plan builds on it as written.
The reviews raised two choices for the owner, about a store that does not open and about folders that a group can write, and the section "For the owner" describes them.
Several choices where the design is silent are logged as assumptions, and the owner can change them later without rework: the folder for repositories, the agent limit, the size of the event buffer, how long receipts are kept, and the formula for the load on macOS.

## Key decisions

**A full event buffer refuses new events.**
The design says that no event is lost.
Dropping the oldest event would break that and leave a gap in the numbers.
The program that makes an event therefore gets an error and decides what to do.

**Every event is on disk before it gets a number that can be sent.**
If an event could be sent before it was safe on disk, a crash could give its number to a different event.
The server would then drop that event as a repeat.
Each event costs one commit of the store, which is fine for the few events of wave 1.

**A command runs at most once, even across a crash.**
The client writes a receipt that says "started" before it runs a command.
If the client stops in the middle, that receipt becomes "interrupted" when the client starts again, and the command is not run again.
The server can send a new command if it still wants the work done.
A damaged receipt is kept for ever as "interrupted", so that no cleanup can make room for a second run (critic-r1 4).

**The outbox deletes only events that were sent and that the server applied.**
The store records the highest number the sender is about to send, before it sends it.
An acknowledgement above that number is refused, so the server can never delete an event the client did not send (architect-r2 1).
When the client connects, the server's number is checked once. A server number above what was sent means a new or restored store, and the unsent events then move to follow the server's number instead of being deleted.
Round 2 dropped the epoch of round 1, so the plan needs no new field in the contract (UNREVIEWED MECHANISM).

**The harness scan never runs a shell and never inherits secrets.**
It runs each program directly with the one argument `--version`, with only three variables in its environment, and with a time limit of 3 seconds.
It counts a program as installed only when its whole first line has the exact form of that harness's version, so nothing else a program prints can reach the website (threat-r1 10).
It never runs a program from a place that another user can change (threat-r1 06).
Tests use fake programs in a temporary folder, so they never touch the real programs on the machine.

**The load on macOS uses the load average.**
macOS gives no busy time of the processor to a Go program without a C library call.
The client therefore reports the load of the last minute divided by the number of cores, on Linux and on macOS alike.
A test compares the result with what macOS itself reports, on a real Mac in phase 5 of sprint 1, or of sprint 3 when no Mac is available in sprint 1 (critic-r1 9).

**The plan needs one module outside Go's standard library.**
The store `go.etcd.io/bbolt` is that module, by the ruling of round 2. It builds for Linux and macOS without a C compiler.
The module file `go.mod` belongs to the Contract team, so the result file asks for it.

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
It refuses to load a key file that other users can read, that is a link to another file or not a plain file, that another user owns, or that holds anything but one key.
It never overwrites an existing key, because that would erase the machine's identity.
The risk is a key that leaks into a log or a file that others can read, and tests try each of these.
When it is done, the key tests pass, and a reviewer who removes the permission check sees them fail.

### The client can describe its machine and its harnesses

The client reads the facts that the Machines page of the website shows: the host name, the operating system, the cores, the memory, its own version, and the load of the processor and of the memory.
It also looks for the four harnesses of the design and reads the version of each one that is installed.
The main risks are a program that never answers, and a program that prints a secret.
A time limit handles the first, and accepting only a line with the exact form of a version handles the second.
The scan also skips a program in a folder that another user can change.
When it is done, the tests pass with fake programs, and no real harness on the machine is run.

### Nothing the client must send or has done is lost in a crash

The client keeps the events that wait for the server in a store on disk, numbered from 1, until the server acknowledges them.
It deletes an event only when it was sent and the server applied it.
The same store keeps the receipts, and each change to it is one commit.
It keeps a receipt of each command, so that a command that arrives twice runs once and the first result is sent again.
A command that arrives after its expiry is not run.
The main risk is a crash in the middle of a write, which could lose an event, reuse its number, or run a command twice.
Tests kill a child process again and again while it writes, and they check after each kill that every event that got a number is whole and that no command can run again.
When it is done, those tests pass, and a reviewer who moves the write after the run sees them fail.

## Rollout order

1. Wait for `go.mod` from plan `go-s0-contract`, because no Go code builds without it. Items 6 and 7 also wait for `go.etcd.io/bbolt` in it.
2. Build items 1, 2, 4 and 5 at the same time. They share no file and need nothing from each other.
3. Build items 3 and 6 when item 2 is merged, because the key is written through item 2 and the store claims the folder of item 2.
4. Build item 7 when item 6 is merged, because the receipts live in the store of item 6.
5. Run the whole suite of the client, the build for macOS and the post-build checks on the merged work, in phase 5 of the build loop.
6. Hand the layout of the state folder and the section "What plan go-s3-client-link must do" to plan `go-s3-client-link` before its planner freezes it.

Nothing in this plan is hard to reverse.
No step needs the owner.

## For the owner

Review round 2 leaves two choices that the design does not settle.
Neither blocks the build of sprint 1, because the plan takes the safe side of each until the owner rules.

**What happens when the store of events and receipts does not open.**
Since round 2, the events and the receipts live in one store file, `client.db`.
A store that does not open, for example after a disk fault, now stops the client, and nothing is deleted.
The owner can choose one of three ways forward.

1. Keep refusing, as the plan does now. Nothing is ever lost silently, but the machine stays off the website until a person moves the file aside by hand. This costs nothing to build.
2. Move the file to `client.db.damaged-<time>` and start a new store. The machine comes back by itself, and its reports are made again. The events in the old file are lost, and a command that was in flight could run a second time, because its receipt is in the old file. This costs about half a day in plan `go-s3-client-link`.
3. Refuse until a person runs a repair command, which then does what option 2 does. A person sees every loss before it happens. This costs one more command in plan `go-s3-client-link`.

**Whether the harness scan trusts a folder that a group can write.**
The scan runs a harness only from a folder that no other user can change.
On a Mac with Homebrew in `/usr/local`, that folder is often writable by the group `admin`, so a harness installed there would show as not installed.
The owner can choose how strict the scan is.

1. Keep the strict rule, as the plan does now. No other user can plant a program that the client runs, but some common installs are not found. This costs nothing.
2. Also trust a folder that the user owns and that only the user's own group can write, when that group has the user's name. This finds installs on systems that give each user a private group, and still refuses `admin`. This costs one check and its tests.
3. Let the owner list trusted groups in `settings.json`. Any member of a listed group could then plant a program that the client runs. This costs one setting and its tests.

**What changed since round 1.**
The choice about a client clock that runs behind the server is gone: the draft contract now lets the client count the server's time, so the client's own clock reaches no decision.
The promise that buffered events arrive once and in order was stuck again in round 2.
By a ruling of the orchestrator, its mechanism was replaced by a store with transactions, and round 3 attacks it as a new design.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**The rules that every item of this plan keeps**

- The team edits only `src/client/`. Every file named below is under it.
- The module path is `github.com/qnlbnsl/lgtm-studio`, so the import path of a package is `github.com/qnlbnsl/lgtm-studio/src/client/internal/<name>`.
- Only the standard library of Go is imported, except `go.etcd.io/bbolt` in items 6 and 7.
- No test needs a server, a database or the network. `go test ./src/client/...` passes with the network turned off.
- Every test also passes under `go test -race`.
- The client builds for Linux and macOS with `CGO_ENABLED=0`. A file that only one system can compile carries a build line, `//go:build linux` or `//go:build darwin`.
- A test that needs a crash runs the test binary again as a child process. The child is started with `os.Executable()`, the argument `-test.run=^<helper>$`, and the variable `LGTM_TEST_HELPER` naming what it does. Each package has one helper test with its own name: `TestHelperStateProcess` for item 2, `TestHelperStoreProcess` in `store_crash_test.go` for item 6, and `TestHelperReceiptsProcess` in `receipts_crash_test.go` for item 7, so that two test writers never define the same name or edit the same file (critic-r1 6). A helper returns at once when `LGTM_TEST_HELPER` does not name one of its modes, so it never runs on its own.
- The workstreams are numbered WS-1 to WS-4, in the order of the section "Workstreams" above.
- Every work item has the scope `client/backend` and the work area `backend`.

**The layout of the state folder**

Plan `go-s3-client-link` reads this layout. It is the agreement of point 4 in the epic's section "What the teams must agree on".
Round 2 replaced the folders `outbox/` and `receipts/` with one store file, by a ruling of the orchestrator of 2026-11-26. UNREVIEWED MECHANISM.

| Path inside the folder | Kind and mode | Format | Written by | Read by |
|---|---|---|---|---|
| (the folder) | folder, `0700` | | item 2, `state.Open` | every item |
| `lock` | file, `0600` | An empty regular file. A client holds an exclusive `flock` on it for as long as it uses the folder. `state.Open` refuses a `lock` that is a link, is not a regular file, has a bit for group or others, or belongs to another user (threat-r1 04). | item 2 | item 2 |
| `settings.json` | file, `0600` | JSON (JavaScript Object Notation): `{"version":1,"repositories_folder":"~/…","agent_limit":4}`. Both fields after `version` are optional. | the owner or the installer | item 1, `config.LoadSettings` |
| `machine.key` | file, `0600` | one PEM (Privacy-Enhanced Mail) block of type `PRIVATE KEY`, holding the Ed25519 key in PKCS #8 (Public-Key Cryptography Standards number 8) form, and at most one newline after it | item 3, `enroll.Create` | item 3, `enroll.Load` |
| `identity.json` | file, `0600` | JSON that plan `go-s3-client-link` defines. The user-flow test `deploy/test/checks.sh` reads this name and its fields `clientPublicKey` and `serverPublicKey`, which hold the two public keys in the same encoding. Plan `go-s3-client-link` writes it last in a join, and only after the server confirmed the join. A `machine.key` without an `identity.json` therefore means a join that has not finished (architect-r1 7). Round 2 renamed it from `server.json` to match the user-flow test. | plan `go-s3-client-link`, through `state.WriteAtomic` | plan `go-s3-client-link`, and the user-flow test |
| `client.db` | file, `0600` | One store of `go.etcd.io/bbolt` with three buckets, `meta`, `events` and `receipts`, described in item 6. It holds the events that wait for the server and the receipts of commands. It is made under a temporary name and linked to `client.db` only when it is complete. UNREVIEWED MECHANISM. | item 6 | items 6 and 7 |
| `repositories/` | folder, `0700` | The default folder for clones. Item 1 only names it. Wave 2 creates and fills it (logic2-r1 12). | wave 2 | wave 2 |
| `.tmp-<random>` and `.tmp-<random>.db` | file, `0600` | an unfinished write, or a store that was not yet complete. Every opener removes it while it holds the lock. | items 2 and 6 | nobody |

The folder is found by this rule, from decision 8:

1. `LGTM_CLIENT_STATE_DIR`, when it is set and not empty. It must be an absolute path.
2. Otherwise `$XDG_STATE_HOME/lgtm-client`, when `XDG_STATE_HOME` is an absolute path. XDG is the X Desktop Group, whose rule for folders this follows.
3. Otherwise `$HOME/.local/state/lgtm-client`. This is the folder the design names.

Every folder above the state folder must be safe from other users, as item 2 checks (threat-r1 04).
The installer, `join`, `run` and a person at a shell all use this rule with the same environment, so they reach the same folder (architect-r2 12).

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
- **Design statements:** D2, D16 (the program has no command that serves the web), D19 (the folder for clones), R1, and the section "The client in wave 1".
- **Risky:** no.
- **Public names that others build against:**
  - In `main`: `func run(args []string, stdout, stderr io.Writer) int`, and `var version = "0.0.0-dev"`, which an installer sets with `-ldflags "-X main.version=<v>"`.
  - In `config`: `const StateDirVariable = "LGTM_CLIENT_STATE_DIR"`; `func StateDir(getenv func(string) string) (string, error)`; `type Settings struct { RepositoriesFolder string; AgentLimit uint32 }`; `func DefaultSettings(stateDir string) Settings`; `func LoadSettings(path, home, stateDir string) (Settings, error)`; `var ErrNoStateDir`, `var ErrSettingsInvalid`. The two functions of settings take the state folder since round 1 (logic2-r1 12). Writer: item 1. Readers: the `run` command of plan `go-s3-client-link`, and wave 2.
- **Behaviour:**
  - `run` with `version` writes the version and a newline to `stdout` and returns 0.
  - `run` with `join` or `run` writes one sentence to `stderr` that names the command and says it is not built yet, and returns 1.
  - `run` with no argument or an unknown one writes the usage to `stderr` and returns 2.
  - `main` calls `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))`.
  - `StateDir` follows the three steps of the layout above. A relative `LGTM_CLIENT_STATE_DIR` is an error. A relative `XDG_STATE_HOME` is skipped. No usable value returns `ErrNoStateDir`.
  - `DefaultSettings(stateDir)` returns the folder `<stateDir>/repositories` and the agent limit 4 (logic2-r1 12). The clones then live in the same private folder as the rest of the state, and a second state folder on the same machine gets its own clones. UNREVIEWED MECHANISM: the reviewer proposed a folder beside the state folder. A folder inside it is simpler, and it is already private.
  - `LoadSettings` returns the defaults when the file does not exist. It refuses with `ErrSettingsInvalid` a file that is not JSON, a `version` other than 1, an unknown field, a folder that is neither absolute nor starts with `~/`, and an agent limit outside 1 to 64. A folder that starts with `~/` is expanded against `home`.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `main_test.go` | `TestVersionPrintsVersion` | `run([]string{"version"}, …)` returns 0, and `stdout` is the value of `version` and a newline. |
| `main_test.go` | `TestJoinAndRunAreNotBuiltYet` | For `join` and for `run`, the return is 1, `stdout` is empty, and `stderr` holds the command's name and the words `not built yet`. |
| `main_test.go` | `TestUnknownCommandPrintsUsage` | For no argument and for `serve`, the return is 2, and `stderr` names the three commands `join`, `run` and `version`. |
| `build_test.go` | `TestBuildsForLinuxAndMacOS` | Skipped under `-short`. For each pair of `linux` or `darwin` with `amd64` or `arm64`, it runs two builds from the module root with `CGO_ENABLED=0`: `go build ./src/client/...`, which checks every package and writes nothing, and `go build -o <temp>/lgtm-client ./src/client/cmd/lgtm-client`, which writes the program. Each build exits 0, and the program file exists afterwards (architect-r1 10). The test fails, and does not skip, when `go` is not on the `PATH`. |
| `config_test.go` | `TestStateDirPrefersVariable` | With `LGTM_CLIENT_STATE_DIR=/a/b`, `XDG_STATE_HOME=/x` and `HOME=/h`, the result is `/a/b`. |
| `config_test.go` | `TestStateDirRefusesRelativeVariable` | With `LGTM_CLIENT_STATE_DIR=rel`, the error is not nil. |
| `config_test.go` | `TestStateDirUsesXDGStateHome` | With `XDG_STATE_HOME=/x` and `HOME=/h`, the result is `/x/lgtm-client`. |
| `config_test.go` | `TestStateDirSkipsRelativeXDG` | With `XDG_STATE_HOME=rel` and `HOME=/h`, the result is `/h/.local/state/lgtm-client`. |
| `config_test.go` | `TestStateDirWithoutHome` | With no variable set, the error is `ErrNoStateDir`. |
| `config_test.go` | `TestLoadSettingsMissingFileGivesDefaults` | A path that does not exist gives `DefaultSettings(stateDir)`, whose folder is `<stateDir>/repositories`, and no error (logic2-r1 12). |
| `config_test.go` | `TestLoadSettingsReadsFields` | `{"version":1,"repositories_folder":"~/repos","agent_limit":8}` gives `<home>/repos` and 8. |
| `config_test.go` | `TestLoadSettingsRefusesBadFiles` | Each of these gives `ErrSettingsInvalid`: text that is not JSON, `"version":2`, an unknown field `"agentLimit"`, the folder `repos`, the agent limit 0, and the agent limit 65. |

- **Acceptance:** `go test ./src/client/cmd/... ./src/client/internal/config/...` passes. `go run ./src/client/cmd/lgtm-client version` prints `0.0.0-dev`.

**Item 2. The state folder and safe writes**

- **Files:** `src/client/internal/state/folder.go`, `src/client/internal/state/atomic.go`, `src/client/internal/state/testhook.go`, `src/client/internal/state/folder_test.go`, `src/client/internal/state/atomic_test.go`, `src/client/internal/state/crash_test.go`.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 4 and 5.
- **Work area:** `backend`. **Complexity:** medium, because the method of writing is known and fits in two small files, and each check of the folder has its own test.
- **Scope:** `client/backend`.
- **Design statements:** D19 (the state belongs to one user), R9 (no other user reads the state), the section "Joining" step 3 (a file only its user can read), and the sections "Commands go down" and "Events come up", whose store lives in this folder.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Writing the final file in place instead of through a temporary file makes `TestWriteAtomicCrashAtEachStep` fail.
  2. Removing the sync of the temporary file makes `TestWriteAtomicSyncsBeforeRename` fail.
  3. Removing the check of group and other bits in `Open` makes `TestOpenRefusesExposedFolder` fail.
  4. Removing the `flock` makes `TestOpenRefusesSecondHolder` fail.
  5. Using a rename instead of a link in `CreateExclusive` makes `TestCreateExclusiveNeverReplaces` fail.
  6. Removing the sync of the parent of a new folder makes `TestOpenSyncsNewFolderEntries` fail (architect-r1 1).
  7. Returning a plain error when the folder sync fails after placement makes `TestWriteAtomicReportsPlacedNotSynced` fail (critic-r1 1).
  8. Removing the check of the parent folders makes `TestOpenRefusesWritableParent` fail (threat-r1 04).
  9. Removing the claim of a store makes `TestClaimAllowsOneStoreEach` fail (logic-r1 3).
  10. Syncing on a reopen only the parent of the state folder, instead of every folder of the chain, makes `TestOpenSyncsWholeChainOnReopen` fail (architect-r2 2, logic-r2 03).
  11. Taking the store's path from a value the caller can change makes `TestClaimUsesTheLockedFolder` fail (logic-r2 10).
- **Public names that others build against:**
  - Constants `LockFile = "lock"`, `SettingsFile = "settings.json"`, `KeyFile = "machine.key"`, `IdentityFile = "identity.json"`, `StoreFile = "client.db"`, `RepositoriesDir = "repositories"`, `TempPrefix = ".tmp-"`. Round 2 removed `OutboxDir` and `ReceiptsDir`, and renamed `ServerFile = "server.json"` to `IdentityFile`, the name that the user-flow test `deploy/test/checks.sh` reads. Writer: item 2. Readers: items 3 and 6, and plan `go-s3-client-link`.
  - `type Paths struct { Dir, Lock, Settings, Key, Identity, Store, Repositories string }` and `func PathsIn(dir string) Paths`.
  - `type Folder struct`, whose fields are all unexported, with `func Open(dir string) (*Folder, error)`, `func (f *Folder) Paths() Paths`, which returns a copy, and `func (f *Folder) Close() error` (logic-r2 10).
  - `func (f *Folder) Claim(name string) (Paths, func(), error)`. It returns the paths of the folder it locked, and a function that releases the claim. Item 6 calls it with `"store"`. A second claim of the same name before its release returns `ErrStoreOpen`. A claim on a `nil` folder, on a zero `Folder`, or after `Close` returns `ErrFolderClosed`. `Close` returns `ErrStoreOpen` and keeps the lock while a claim is held. Writer: item 2. Reader: item 6 (logic-r1 3, logic-r2 10).
  - `func WriteAtomic(dir, name string, data []byte) error`, which writes mode `0600`.
  - `func CreateExclusive(dir, name string, data []byte) error`, which writes mode `0600` and returns an error that wraps `os.ErrExist` when the name exists.
  - `func RemoveTemps(dir string) error` and `func SyncDir(dir string) error`.
  - `var ErrNotAFolder`, `ErrFolderExposed`, `ErrFolderNotOwned`, `ErrFolderLocked`, `ErrFolderClosed`, `ErrBadName`, `ErrParentUnsafe`, `ErrLockUnsafe`, `ErrStoreOpen`, `ErrPlacedNotSynced`. Writer: item 2. Readers: items 3 and 6, and plan `go-s3-client-link`.
  - Unexported, for tests in the package only: `var crashHook func(step string)`, `var syncFile func(*os.File) error`, `var syncFolder func(dir string) error`. The steps of a write are `"temp-created"`, `"temp-written"`, `"temp-synced"`, `"placed"` and `"dir-synced"`, in that order. `"placed"` is the rename, or the link for `CreateExclusive`. `Open` also calls the hook with `"folder-created"` after it creates a folder and before it syncs that folder's parent.
  - In `testhook.go`: `func SetCrashHookForTest(hook func(step string)) (restore func())` and `func SetSyncFolderForTest(sync func(dir string) error) (restore func())`. Each sets its variable and returns a function that puts the old one back. Their comments say that only tests call them.
- **Behaviour:**
  - `Open` first resolves the part of the path that exists with `filepath.EvalSymlinks`. Every folder of that resolved path, from the root down, must belong to the user who runs the client or to the superuser. It must have no write bit for group or others, unless it belongs to the superuser and has the sticky bit, as `/tmp` does. Otherwise `Open` returns `ErrParentUnsafe` and creates nothing (threat-r1 04).
  - `Open` then creates each missing folder of the path, one at a time from the top, with mode `0700`. After it creates a folder, it syncs that folder's parent (architect-r1 1).
  - `Open` refuses the state folder when it is a symbolic link or not a folder (`ErrNotAFolder`), when its mode has any bit of group or others (`ErrFolderExposed`), and when its owner is not the user who runs the client (`ErrFolderNotOwned`). It never changes the mode of a folder that already exists.
  - `Open` opens `lock` with `O_RDWR|O_CREATE|O_NOFOLLOW` and mode `0600`. It checks the open file with `Stat`: a file that is not regular, has a bit of group or others, or belongs to another user gives `ErrLockUnsafe`. It then takes `syscall.Flock` with `LOCK_EX|LOCK_NB`. When another holder has it, `Open` returns `ErrFolderLocked`. The open file stays open until `Close`.
  - Under the lock, `Open` calls `RemoveTemps` on the state folder.
  - Before it returns, `Open` always syncs every folder of the resolved chain, from the root down to the state folder, even when it created nothing (architect-r2 2, logic-r2 03). A later `Open` therefore makes durable every entry that an earlier `Open` created and did not sync, whichever folder that was. Any failed sync makes `Open` release the lock and return the error, and the caller gets no `Folder`.
  - `WriteAtomic` and `CreateExclusive` refuse with `ErrBadName` a name that is empty, `.` or `..`, holds a path separator, or starts with `TempPrefix`.
  - `WriteAtomic` creates `.tmp-<16 random hexadecimal characters>` with `O_CREATE|O_EXCL` and mode `0600`, writes the data, syncs the file, closes it, renames it over the name, and syncs the folder.
  - `CreateExclusive` does the same, except that it links the temporary file to the name with `os.Link`, which fails when the name exists, and then removes the temporary file.
  - Both have two kinds of failure (critic-r1 1). A failure before the step `placed` changes nothing: the temporary file is removed and the final name is as before. A failure of the folder sync after `placed` returns an error that wraps `ErrPlacedNotSynced`. The new file is then in place in the running system, and it may or may not survive a loss of power. Item 3 and the fold "What plan go-s3-client-link must do" say what each caller does with it.
  - Each error message is one sentence that names the folder or the file.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `folder_test.go` | `TestOpenCreatesPrivateFolders` | `Open` on a path inside `t.TempDir()` that does not exist creates the folder with mode exactly `0700`, and `lock` with mode exactly `0600`. |
| `folder_test.go` | `TestPathsInNamesEveryFile` | `PathsIn("/s")` gives `/s/lock`, `/s/settings.json`, `/s/machine.key`, `/s/identity.json`, `/s/client.db` and `/s/repositories`. |
| `folder_test.go` | `TestOpenRefusesExposedFolder` | For an existing folder of mode `0750`, then `0705`, `Open` returns `ErrFolderExposed`, and the mode is unchanged afterwards. |
| `folder_test.go` | `TestOpenRefusesSymlink` | When the path is a symbolic link to a folder of mode `0700`, `Open` returns `ErrNotAFolder`. |
| `folder_test.go` | `TestOpenRefusesWritableParent` | When a parent of the state folder has mode `0777` without the sticky bit, `Open` returns `ErrParentUnsafe`, and nothing is created inside it (threat-r1 04). |
| `folder_test.go` | `TestOpenRefusesUnsafeLock` | An existing `lock` that is a symbolic link, and one with mode `0644`, each give `ErrLockUnsafe`. |
| `folder_test.go` | `TestOpenSyncsNewFolderEntries` | With `syncFolder` replaced by a recorder, `Open` of `<tmp>/a/b/state`, where `a` and `b` do not exist, records a sync of `<tmp>`, `<tmp>/a` and `<tmp>/a/b` after each creation, and then of every folder from the root down to the state folder. With `syncFolder` failing for `<tmp>/a`, `Open` returns an error and holds no lock. |
| `folder_test.go` | `TestOpenSyncsWholeChainOnReopen` | The first `Open` of `<tmp>/a/b/state` creates `a` and `b` and then fails on the sync of `<tmp>/a`. A second `Open` with a working sync succeeds, and its recorded syncs hold `<tmp>`, `<tmp>/a`, `<tmp>/a/b` and the state folder (architect-r2 2, logic-r2 03). |
| `folder_test.go` | `TestOpenRefusesSecondHolder` | A second `Open` of the same folder, while the first is open, returns `ErrFolderLocked`. After the first `Close`, a new `Open` succeeds. The second `Open` also runs in a child process. |
| `folder_test.go` | `TestLockIsFreedWhenHolderDies` | A child holds the folder open. After the parent kills it with the signal `SIGKILL`, `Open` in the parent succeeds. |
| `folder_test.go` | `TestClaimAllowsOneStoreEach` | A second `Claim("store")` before the release returns `ErrStoreOpen`. After the release, a claim succeeds. `Close` while a claim is held returns `ErrStoreOpen`. |
| `folder_test.go` | `TestClaimUsesTheLockedFolder` | `Claim` returns the paths of the folder that `Open` locked, and changing the copy that `Paths()` returned does not change them. `Claim` on a zero `Folder` and after `Close` returns `ErrFolderClosed`. With `<tmp>/alias` a link to `<tmp>/real`, `Open` of `<tmp>/alias/state` while `<tmp>/real/state` is open returns `ErrFolderLocked` (logic-r2 10). |
| `folder_test.go` | `TestOpenRemovesTemps` | A file `.tmp-abc` and a file `.tmp-abc.db` in the folder are gone after `Open`. A file `keep.json` stays. |
| `atomic_test.go` | `TestWriteAtomicWritesPrivateFile` | After `WriteAtomic(dir, "a.json", data)`, the file holds exactly `data`, its mode is exactly `0600`, and no name with the prefix `.tmp-` is left. |
| `atomic_test.go` | `TestWriteAtomicReplaces` | Writing `A` and then `B` leaves exactly `B`. |
| `atomic_test.go` | `TestWriteAtomicRefusesBadNames` | `""`, `"."`, `".."`, `"a/b"` and `".tmp-x"` each return `ErrBadName`, and the folder is unchanged. |
| `atomic_test.go` | `TestWriteAtomicSyncsBeforeRename` | With `crashHook`, `syncFile` and `syncFolder` replaced by recorders, the recorded order is `temp-created`, `temp-written`, the file sync, `temp-synced`, `placed`, the folder sync, `dir-synced`. The same order holds for `CreateExclusive`. |
| `atomic_test.go` | `TestWriteAtomicReportsPlacedNotSynced` | With `syncFolder` failing, `WriteAtomic` of `B` over `A` returns an error for which `errors.Is(err, ErrPlacedNotSynced)` is true, and the file holds `B`. With `syncFile` failing instead, the error is not `ErrPlacedNotSynced`, the file holds `A`, and no `.tmp-` name is left. |
| `atomic_test.go` | `TestCreateExclusiveReportsPlacedNotSynced` | With `syncFolder` failing, `CreateExclusive` of `k` returns `ErrPlacedNotSynced`, and `k` holds the data. |
| `atomic_test.go` | `TestCreateExclusiveNeverReplaces` | `CreateExclusive` of `k` with `A` succeeds. A second call with `B` returns an error for which `errors.Is(err, os.ErrExist)` is true, and the file still holds exactly `A`. |
| `crash_test.go` | `TestWriteAtomicCrashAtEachStep` | For each of the five steps, a child writes `x` with content `A`, then writes `x` with content `B` of 256 kibibytes, with `crashHook` set to `os.Exit(86)` at that step. The parent asserts that the child exited with 86. For `temp-created`, `temp-written` and `temp-synced`, `x` holds exactly `A`. For `placed` and `dir-synced`, `x` holds exactly `B`. |
| `crash_test.go` | `TestCreateExclusiveCrashAtEachStep` | For each step, a child creates `k` with content `A` and exits with 86 at that step. For the first three steps, `k` is missing. For `placed` and `dir-synced`, it holds exactly `A`. |
| `crash_test.go` | `TestOpenCrashAfterCreatingParent` | A child opens `<tmp>/a/b/state` and exits with 86 at the step `folder-created` for `a`. A second `Open` in the parent succeeds and records a sync of `<tmp>` (architect-r2 2). |

The child processes of this item run the helper test `TestHelperStateProcess` in `crash_test.go`.
A process kill leaves the operating system running, so these tests prove the order of the steps and not survival of a loss of power. A post-build check covers the rest.

- **Acceptance:** `go test -race ./src/client/internal/state/...` passes. The break reviewer's report names the eleven mutations above and the test that went red for each.

**Tasks of WS-1:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1.1 | Write the commands of the program | `src/client/cmd/lgtm-client/main.go`, `run` (target). Pattern: `packages/cli/src/bin.ts`. |
| 1.2 | Write the cross build test | `src/client/cmd/lgtm-client/build_test.go`, `TestBuildsForLinuxAndMacOS` (target). Pattern: `deploy/test/checks.sh`, function `machines`, which builds the same package with `go build -o` (critic-r1 12). |
| 1.3 | Write the finder of the state folder and the settings reader | `src/client/internal/config/config.go`, `StateDir`, `DefaultSettings` and `LoadSettings` (target). Pattern: `floorplan/plan.json`, entry "Machine settings". |
| 2.1 | Write the checks, the lock and the claims of the folder | `src/client/internal/state/folder.go`, `Open` and `Claim` (target). Pattern: `packages/engine/src/harnesses/programs.ts`, `recordManagedInstallation`. |
| 2.2 | Write the safe writes | `src/client/internal/state/atomic.go`, `WriteAtomic` and `CreateExclusive` (target). Pattern: `packages/engine/src/workflows/store.ts`, class `WorkflowStore`, whose save links a new file and renames over an old one (critic-r1 12). |
| 2.3 | Write the exported test hooks | `src/client/internal/state/testhook.go`, `SetCrashHookForTest` and `SetSyncFolderForTest` (target). Pattern: the unexported variables `crashHook` and `syncFolder` of task 2.2. |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The program answers its three commands. | `go test -run 'TestVersionPrintsVersion|TestJoinAndRunAreNotBuiltYet|TestUnknownCommandPrintsUsage' ./src/client/cmd/lgtm-client/` passes. |
| 2 | The program builds for Linux and macOS. | `go test -run TestBuildsForLinuxAndMacOS ./src/client/cmd/lgtm-client/` passes. It runs again on the merged work of all seven items in phase 5 (critic-r1 answer 5). |
| 3 | The state folder is created private, and a folder that others can read or change is refused. | `go test -run 'TestOpenCreatesPrivateFolders|TestOpenRefusesExposedFolder|TestOpenRefusesSymlink|TestOpenRefusesWritableParent|TestOpenRefusesUnsafeLock' ./src/client/internal/state/` passes. |
| 4 | Only one client uses a state folder, and only one store of each kind is open in it. | `go test -run 'TestOpenRefusesSecondHolder|TestLockIsFreedWhenHolderDies|TestClaimAllowsOneStoreEach|TestClaimUsesTheLockedFolder' ./src/client/internal/state/` passes. |
| 5 | A crash in a write leaves the old file or the new one. | `go test -run 'TestWriteAtomicCrashAtEachStep|TestCreateExclusiveCrashAtEachStep' ./src/client/internal/state/` passes. |
| 6 | Every new folder of the chain is made durable, also by a later open, and a write whose folder sync failed is reported, not hidden. | `go test -run 'TestOpenSyncsNewFolderEntries|TestOpenSyncsWholeChainOnReopen|TestOpenCrashAfterCreatingParent|TestWriteAtomicReportsPlacedNotSynced|TestCreateExclusiveReportsPlacedNotSynced' ./src/client/internal/state/` passes. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A crash leaves a half-written file that a later reader trusts. | high | The temporary file and the rename of item 2, proven by `TestWriteAtomicCrashAtEachStep`. |
| A loss of power undoes a rename, or a new folder, that was not synced. | high | The folder is synced after every rename, and the parent after every new folder, proven by `TestWriteAtomicSyncsBeforeRename` and `TestOpenSyncsNewFolderEntries`. A failed folder sync after the rename is returned as `ErrPlacedNotSynced`, so no caller treats the write as certain. A real loss of power is a post-build check. |
| Another user swaps a folder above the state folder. | high | `Open` refuses a parent that another user can change, proven by `TestOpenRefusesWritableParent` (threat-r1 04). |
| A folder sync behaves differently on macOS. | medium | Go's `os.File.Sync` uses `F_FULLFSYNC` and falls back to `fsync` on `ENOTSUP`. The tests run on Linux, so phase 5 runs `go test ./src/client/...` on a Mac when one is available, and the gap is logged when none is. |
| The test that builds for four targets is slow. | low | It is skipped under `-short`, and the full suite runs it. |

**Verify:** the tests of items 1 and 2, and `go test -race ./src/client/...`. Device check: none for Linux. A run of `go test ./src/client/internal/state/...` on a Mac is wanted in phase 5. The section "Post-build checks" lists the checks that need the built code.

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
  4. Removing the methods `String`, `GoString` and `Format`, or keeping the key bytes in a field that is not behind a pointer, makes `TestCredentialNeverPrintsKey` fail (logic2-r1 4).
  5. Replacing `state.CreateExclusive` with `state.WriteAtomic` makes `TestCreateRefusesExistingKey` fail.
  6. Opening the file without `O_NONBLOCK` makes `TestLoadRefusesNamedPipe` fail by its time limit (critic-r1 7).
  7. Reading only the first 4 kibibytes without a check of the size makes `TestLoadRefusesOversizedKey` fail (architect-r1 8).
- **Public names that others build against:**
  - `type Credential struct` whose only field is an unexported pointer to an unexported struct that holds the key (logic2-r1 4).
  - `func Create(path string) (*Credential, error)` and `func Load(path string) (*Credential, error)`.
  - `func (c Credential) PublicKey() ed25519.PublicKey`, which returns a copy.
  - `func (c Credential) Sign(message []byte) []byte`.
  - `func (c Credential) Signer() crypto.Signer`, which returns a value whose only methods are `Public` and `Sign`, so a caller cannot reach the private key by a type assertion.
  - `func (c Credential) Fingerprint() string`, which returns `SHA256:` and the unpadded standard base64 of the SHA-256 (Secure Hash Algorithm, 256 bits) hash of the 32-byte public key.
  - `func (c Credential) String() string`, `GoString() string` and `Format(f fmt.State, verb rune)`, all on the value and not the pointer. Each writes `machine key ` and the fingerprint, for every verb (logic2-r1 4).
  - `var ErrKeyExists`, `ErrKeyMissing`, `ErrKeyExposed`, `ErrKeyNotOwned`, `ErrKeyNotRegular`, `ErrKeyInvalid`, `ErrKeyUnreadable`. Writer: item 3. Reader: the `join` and `run` commands of plan `go-s3-client-link`.
- **Behaviour:**
  - `Create` makes the key with `ed25519.GenerateKey(crypto/rand.Reader)`, encodes it with `x509.MarshalPKCS8PrivateKey` into one PEM block of type `PRIVATE KEY`, and writes it with `state.CreateExclusive`. An existing file gives `ErrKeyExists`, which wraps `os.ErrExist`, and the existing file is not changed.
  - When `state.CreateExclusive` returns `ErrPlacedNotSynced`, `Create` returns that error and no credential. The key may be in place. The caller then syncs the folder with `state.SyncDir`, and only after that sync succeeds does it call `Load`, which reads that key. It never makes a second key, and it never sends the key to the server while its placement is uncertain (logic2-r1 8, architect-r2 8).
  - `Load` opens the file with `os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK`, so that a named pipe cannot block it, and checks the open file with `Stat` before it reads (architect-r1 8, critic-r1 7, logic-r1 10, threat-r1 11).
  - A missing file gives `ErrKeyMissing`, and only a missing file gives it. A symbolic link or a file that is not regular gives `ErrKeyNotRegular`. Any bit of group or others gives `ErrKeyExposed`. An owner other than the user who runs the client gives `ErrKeyNotOwned`. Any other failure to open or read gives `ErrKeyUnreadable`, which wraps the cause from the operating system.
  - `Load` reads at most 4,097 bytes. A file of more than 4,096 bytes gives `ErrKeyInvalid`.
  - The content must start with `-----BEGIN PRIVATE KEY-----`. It must hold exactly one PEM block of type `PRIVATE KEY` with an Ed25519 key, and after the block nothing but at most one newline. Anything else gives `ErrKeyInvalid`.
  - No call of `Load` changes the file, whatever it returns.
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
| `credential_test.go` | `TestLoadRefusesNamedPipe` | A named pipe of mode `0600` made with `syscall.Mkfifo`, with no writer, gives `ErrKeyNotRegular` within 1 second (critic-r1 7). |
| `credential_test.go` | `TestLoadReportsUnreadableKey` | A key file of mode `0200` gives `ErrKeyUnreadable`, and `errors.Is(err, fs.ErrPermission)` is true. The test is skipped when it runs as the superuser, who can read any file. |
| `credential_test.go` | `TestLoadRefusesMissingAndInvalid` | A missing path gives `ErrKeyMissing`. A file of mode `0600` holding `hello`, one holding a PEM block of type `PUBLIC KEY`, and one holding an RSA key each give `ErrKeyInvalid`. |
| `credential_test.go` | `TestLoadRefusesOversizedKey` | A valid key followed by 5,000 spaces gives `ErrKeyInvalid` (architect-r1 8). |
| `credential_test.go` | `TestLoadRefusesExtraContent` | A valid key with the line `junk` before it, a valid key followed by a second PEM block, and a valid key followed by `x` each give `ErrKeyInvalid`. A valid key followed by one newline loads. |
| `credential_test.go` | `TestLoadNeverChangesFile` | After each refusal of the tests above, the file's bytes and mode are as before the call. |
| `credential_test.go` | `TestCredentialNeverPrintsKey` | The operands are the credential value, a pointer to it, a struct with an exported field of type `Credential`, and a struct with an unexported field of that type. The verbs are `%v`, `%+v`, `%#v`, `%s`, `%x` and `%q`. No output holds the 32-byte seed in base64, in hexadecimal, as the decimal list that `fmt` prints for a byte slice, or as raw bytes. Neither does `json.Marshal` of the two structs, nor the text of every error the tests above produce. `String()` starts with `machine key SHA256:` (logic2-r1 4). |
| `credential_test.go` | `TestFingerprintFormat` | `Fingerprint()` is `SHA256:` and 43 characters of unpadded standard base64, and it equals that encoding of `sha256.Sum256(PublicKey())`. |

The check of the owner is not given a test, because a test cannot make a file owned by another user without the rights of the superuser. The reviewer reads that branch, and a post-build check runs it.

- **Acceptance:** `go test -race ./src/client/internal/enroll/...` passes. The break reviewer's report names the seven mutations and the test that went red for each.

**Tasks of WS-2:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 3.1 | Write the making and loading of the key | `src/client/internal/enroll/credential.go`, `Create` and `Load` (target). Pattern: the design's section "Joining", step 3. |
| 3.2 | Write the signer that hides the key, and the printing methods | `src/client/internal/enroll/credential.go`, `Signer`, `String`, `GoString` and `Format` (target). Pattern: the interface `crypto.Signer` of Go's standard library, and `Load` of task 3.1 in the same file (critic-r1 12). |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The key file is private, and it is never replaced. | `go test -run 'TestCreateWritesPrivateKeyFile|TestCreateRefusesExistingKey' ./src/client/internal/enroll/` passes. |
| 2 | A key that others could read, or that is not a plain file, is refused. | `go test -run 'TestLoadRefusesExposedKey|TestLoadRefusesSymlink|TestLoadRefusesFolder|TestLoadRefusesNamedPipe' ./src/client/internal/enroll/` passes. |
| 3 | A damaged, oversized or unreadable key is refused with its own error, and the file is left alone. | `go test -run 'TestLoadReportsUnreadableKey|TestLoadRefusesMissingAndInvalid|TestLoadRefusesOversizedKey|TestLoadRefusesExtraContent|TestLoadNeverChangesFile' ./src/client/internal/enroll/` passes. |
| 4 | The key never reaches printed text. | `go test -run TestCredentialNeverPrintsKey ./src/client/internal/enroll/` passes. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The key reaches a log through a formatted print of the credential, or of a struct that holds it. | high | `String`, `GoString` and `Format` on the value return only the fingerprint, and the key sits behind a pointer, proven by `TestCredentialNeverPrintsKey` (logic2-r1 4). |
| A link in the state folder points the key at a file that others can read. | high | `O_NOFOLLOW` and the check of the open file, proven by `TestLoadRefusesSymlink`. |
| A named pipe at the key's path makes the client hang at start. | medium | `O_NONBLOCK` and the check of the open file, proven by `TestLoadRefusesNamedPipe` (critic-r1 7). |
| An agent of the same user reads the key file. | medium | Accepted for now by D19, which makes one user on a machine a convenience. The design plans a user of the client's own later. |
| A second `join` replaces the key and erases the machine's identity. | medium | `CreateExclusive` never replaces a file, proven by `TestCreateRefusesExistingKey`. The section "What plan go-s3-client-link must do" says how a join reuses a key (architect-r1 7). |

**Verify:** the tests of item 3. Device check: none. The check of the owner runs as a post-build check.

**Rollout detail:** step 3 of the rollout order.

**WS-3: The client can describe its machine and its harnesses**

This workstream reads what the design's table "What travels in wave 1" sends up as facts, harnesses found and heartbeat.
Item 4 reads the machine. Item 5 finds the harnesses.
Neither sends anything. Plan `go-s3-client-link` puts their values into the link's messages.
The types follow the fields of message `Client` in `src/contracts/proto/lgtm/v1/clients.proto`, so that the website shows the same meaning.

**Item 4. The facts and the load of the machine**

- **Files:** `src/client/internal/sysinfo/sysinfo.go`, `src/client/internal/sysinfo/parse.go`, `src/client/internal/sysinfo/sysinfo_linux.go`, `src/client/internal/sysinfo/sysinfo_darwin.go`, `src/client/internal/sysinfo/sysinfo_other.go`, `src/client/internal/sysinfo/parse_test.go`, `src/client/internal/sysinfo/sysinfo_linux_test.go`, `src/client/internal/sysinfo/sysinfo_darwin_test.go`, `src/client/internal/sysinfo/testdata/meminfo`, `src/client/internal/sysinfo/testdata/loadavg` (critic-r1 12).
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
| `sysinfo_darwin_test.go` | `TestReadLoadOnMacMatchesSystem` | Runs only on macOS. It reads `ReadLoad()` and `ReadFacts()`, and runs `/usr/sbin/sysctl -n` for `vm.loadavg`, `hw.memsize` and `kern.memorystatus_level`. `MemoryBytes` equals `hw.memsize` exactly. `CPUPercent` is within 2 of `cpuPercent` of the first value of `vm.loadavg` and `runtime.NumCPU()`. `MemoryPercent` is within 2 of 100 minus `kern.memorystatus_level` (critic-r1 9). |
| `sysinfo_linux_test.go` | `TestReadLoadOnLinux` | With `procDir` set to `testdata/`, a fixture load of 2.0 and `runtime.NumCPU()` cores give `cpuPercent(2.0, NumCPU)`, and the memory fixture gives 75. |

- **Acceptance:** `go test ./src/client/internal/sysinfo/...` passes on Linux. `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./src/client/internal/sysinfo/...` exits 0. On a Mac, `go test -run TestReadLoadOnMacMatchesSystem ./src/client/internal/sysinfo/` passes, in phase 5 of sprint 1, or of sprint 3 when no Mac is available in sprint 1.

**Item 5. The harness scan**

- **Files:** `src/client/internal/harness/registry.go`, `src/client/internal/harness/scan.go`, `src/client/internal/harness/trust.go`, `src/client/internal/harness/trust_linux.go`, `src/client/internal/harness/trust_other.go`, `src/client/internal/harness/registry_test.go`, `src/client/internal/harness/scan_test.go`, `src/client/internal/harness/trust_test.go`, `src/client/internal/harness/trust_linux_test.go`.
- **Needs first:** `go.mod`. Runs at the same time as items 1, 2 and 4.
- **Work area:** `backend`. **Complexity:** medium, because it starts child processes under a time limit, which follows a known pattern with clear tests. Round 1 added a check of where a program lives, which is one more file with its own tests.
- **Scope:** `client/backend`.
- **Design statements:** the table "What travels in wave 1" (harnesses found), the section "The client in wave 1" (the four harnesses), D8, R9 and C7.
- **Risky:** no. It touches no sign-in, key, token or ordering. The reviewer still checks `trust.go` against threat-r1 06.
- **Public names that others build against:**
  - `type Name string` with `const Claude Name = "claude"`, `Codex Name = "codex"`, `Gemini Name = "gemini"`, `Cursor Name = "cursor"`. These are the values of `InstalledHarness.harness` in the contract.
  - `type Definition struct { Name Name; Programs []string; Version *regexp.Regexp }` and `func Definitions() []Definition`, in the order Claude, Codex, Gemini, Cursor.
  - `type Problem string` with `ProblemNone Problem = ""`, `ProblemNotFound = "not found"`, `ProblemUntrusted = "untrusted location"`, `ProblemTimedOut = "timed out"`, `ProblemFailed = "failed to run"`, `ProblemNoVersion = "no version in output"`.
  - `type Found struct { Harness Name; Installed bool; Version string; Path string; Problem Problem }`. `Path` and `Problem` stay on the machine. Plan `go-s3-client-link` sends only `Harness` and, when installed, `Version` (architect-r1 9).
  - `type ScanOptions struct { SearchPath []string; Home string; Timeout time.Duration }`, `const DefaultTimeout = 3 * time.Second`, `const WaitDelay = time.Second`, and `func DefaultScanOptions(getenv func(string) string) ScanOptions`, which splits `PATH` and reads `HOME`.
  - `func Scan(ctx context.Context, opts ScanOptions) []Found`.
  - In `trust.go`: `func trustedFolder(dir string) error` and `func trustedProgram(path string) (string, error)`, unexported. The second returns the real path of a trusted program. In `trust_linux.go`: `func hasAccessList(path string) (bool, error)`, and in `trust_other.go` the same function, which returns false.
- **Behaviour:**
  - The programs are `claude` for Claude, `codex` for Codex, `gemini` for Gemini, and `cursor-agent` then `agent` for Cursor.
  - `Scan` first cleans `SearchPath`: it drops empty and relative entries, repeats, and every folder that `trustedFolder` refuses. It uses this trusted list both to find candidates and as the `PATH` of each child, so a program that looks up a helper by name finds it only in a trusted folder (architect-r2 5, logic-r2 07).
  - For each program name in order, and for each folder of the cleaned list in order, a candidate is the folder joined with the name. A candidate that does not exist is skipped silently.
  - A candidate is run only when `trustedProgram` accepts it (threat-r1 06). The rule is the client's own (architect-r2 5). A folder or a file is trusted when it belongs to the user who runs the client or to the superuser, has no write bit for group or others, except a folder of the superuser with the sticky bit, and, on Linux, carries no access list in the extended attribute `system.posix_acl_access`. `trustedFolder` applies this to a folder and to every folder above it, up to the root. `trustedProgram` resolves the candidate one link at a time, and applies the rule to each link, to the folder of each link and of each target, with all the folders above them, and to the final file, which must also be regular and have an execute bit (logic-r2 07). Any failure to read a mode, an owner or an attribute counts as a refusal. A candidate that fails is not run, and it gets `ProblemUntrusted`. UNREVIEWED MECHANISM: round 1 prescribed a trusted location without naming the rule. On macOS, access lists are not checked, and a post-build check records that gap.
  - The rule covers the program that the scan starts and the helpers it finds through `PATH`. It does not cover a file that the program opens by an absolute path, or the libraries that the system's loader finds. A harness that needs such a file must keep it in a trusted folder.
  - A candidate is run with `exec.CommandContext`, its real path, and the one argument `--version`. It is never run through a shell. Its environment is exactly `PATH=<cleaned list joined by the list separator>`, `HOME=<Home>` and `LANG=C`. Its folder is `Home`. Its input is empty. At most 4 kibibytes of its output are read, and its error output is thrown away. `cmd.WaitDelay` is `WaitDelay`.
  - All the candidates of one harness share one deadline of `Timeout`. A `Timeout` of 0 means `DefaultTimeout`. The scan of one harness, and so the whole `Scan`, ends within `Timeout` plus `WaitDelay` plus the time to clean up, because the four harnesses are scanned at the same time (architect-r1 9).
  - A candidate counts as installed only when it exits with status 0, and its whole first line, without the line ending, matches the pattern of its harness (threat-r1 10, critic-r1 8, logic-r1 9). An exit status of 0 together with `exec.ErrWaitDelay`, which a grandchild that holds the output causes, counts as status 0.
  - The patterns match the whole first line. Claude: `^(\d+\.\d+\.\d+) \(Claude Code\)$`. Codex: `^codex-cli (\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)$`. Gemini: `^(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)$`. Cursor: `^(\d{4}\.\d{1,2}\.\d{1,2}-[0-9a-f]{6,12})$`. `Version` is the text of the first group. A harness whose format changes then shows as not installed, and the pattern is updated. Reading the real output of each harness is a post-build check.
  - The first candidate that counts as installed gives `Installed: true`, its real `Path`, and its `Version`.
  - When no candidate counts, `Installed` is false and `Version` is empty. `Problem` is `ProblemNotFound` when no candidate existed. Otherwise it is the problem of the first candidate: `ProblemUntrusted`, `ProblemTimedOut`, `ProblemFailed` for an exit status other than 0, which wins over a matching line, or `ProblemNoVersion`. `Path` is that first candidate.
- **How the tests fake the installed programs:** each test writes small scripts that start with `#!/bin/sh` into folders under `t.TempDir()`, with mode `0755`, and passes only those folders as `SearchPath` and a temporary folder as `Home`. The scan never reads the real `PATH`, so no real harness on the machine is run. A fake that needs another program, such as `sleep`, names it by an absolute path that the test finds once with `exec.LookPath` in the test process, because the child's `PATH` holds only the test folders (architect-r1 9, critic-r1 8, logic-r1 9). A fake that blocks first writes a file `<fake>.started`, and the test asserts that this file exists, so a fake that never started cannot pass. The script is a fake of the harness, and the scan still starts it directly.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `registry_test.go` | `TestDefinitionsOrderAndPrograms` | The names are `claude`, `codex`, `gemini` and `cursor` in that order, and Cursor's programs are `cursor-agent` then `agent`. |
| `scan_test.go` | `TestScanFindsFakePrograms` | Fakes print `2.1.325 (Claude Code)`, `codex-cli 0.98.0`, `0.68.0` and, as `cursor-agent`, `2026.11.20-abc1234`, and exit 0. All four are installed, with the versions `2.1.325`, `0.98.0`, `0.68.0` and `2026.11.20-abc1234`, and each `Path` is the fake's path. |
| `scan_test.go` | `TestScanEmptySearchPathFindsNothing` | With `SearchPath` empty, all four are not installed with `ProblemNotFound`. This holds even when the real machine has a harness on its `PATH`. |
| `scan_test.go` | `TestScanRefusesExtraText` | A fake `claude` prints `2.1.325 (Claude Code) token=sk-planted-secret`. A fake `codex` prints `warning: key 1.2.3-sk-planted` and then `codex-cli 0.98.0` on the second line. Both are not installed with `ProblemNoVersion`, and no field of the result holds `sk-planted` (threat-r1 10). |
| `scan_test.go` | `TestScanPassesOnlyVersionArgument` | A fake writes its arguments to a file beside it. The file holds exactly one line, `--version`. |
| `scan_test.go` | `TestScanGivesMinimalEnvironment` | With `t.Setenv("LGTM_PLANTED_SECRET", "planted")`, a fake writes its environment to a file beside it. The file has no line with `planted`, and its `PATH` and `HOME` lines equal the cleaned search path and the home. |
| `scan_test.go` | `TestScanChildPathIsCleaned` | With `SearchPath` holding `""`, `rel` and a test folder twice, the `PATH` line the fake writes is that test folder alone. |
| `trust_test.go` | `TestScanChildPathHoldsOnlyTrustedFolders` | With `SearchPath` holding a folder of mode `0755` with a fake `claude` and a folder of mode `0777` without the sticky bit, the `PATH` line the fake writes holds only the first folder (architect-r2 5). |
| `trust_test.go` | `TestScanHelperInRefusedFolderIsNotFound` | A fake `claude` in a trusted folder runs the helper `lgtm-helper` by name and prints its output as the version. With the helper only in a folder of mode `0777`, the helper is not found, the fake exits with 127, and Claude is not installed with `ProblemFailed`. With the helper in a trusted folder, Claude is installed (architect-r2 5, logic-r2 07). |
| `trust_test.go` | `TestScanChecksEveryLinkStep` | A link in a trusted folder points at a second link in a folder of mode `0777`, which points at a fake in a trusted folder. The result is `ProblemUntrusted`, and the fake's `.started` file does not exist (logic-r2 07). |
| `trust_linux_test.go` | `TestScanRefusesAccessList` | Runs only on Linux, and is skipped when the program `setfacl` is missing. After `setfacl -m u:nobody:rwx` on the fake's folder, the result is `ProblemUntrusted`. |
| `scan_test.go` | `TestScanNeverUsesShell` | The fake `claude` sits in a folder named `bin $(touch INJECTED);x`. It is found and installed, and no file `INJECTED` exists in `Home` or in the test's folders afterwards. |
| `scan_test.go` | `TestScanSkipsRelativeFolders` | With `SearchPath` holding `.` and the current folder of the test changed with `t.Chdir` to a folder with a fake `claude`, Claude is not installed. This test does not run in parallel. |
| `trust_test.go` | `TestScanSkipsNonExecutable` | A file `codex` of mode `0644` that prints a version gives not installed with `ProblemUntrusted`. |
| `trust_test.go` | `TestScanSkipsUntrustedFolder` | A valid fake in a folder of mode `0777` without the sticky bit is not run: its `.started` file does not exist, and the result is `ProblemUntrusted`. The same fake in a folder of mode `0755` is installed (threat-r1 06). |
| `trust_test.go` | `TestScanSkipsWritableProgram` | A valid fake of mode `0777` is not run and gives `ProblemUntrusted`. |
| `trust_test.go` | `TestScanFollowsLinkToCheck` | A link in a folder of mode `0755` that points at a fake in a folder of mode `0777` gives `ProblemUntrusted`. |
| `scan_test.go` | `TestScanTimesOut` | A fake `gemini` writes its `.started` file and then runs the absolute `sleep 30`. With `Timeout` 500 milliseconds, `Scan` returns within 3 seconds, Gemini is not installed with `ProblemTimedOut`, and the `.started` file exists (critic-r1 8). |
| `scan_test.go` | `TestScanDoesNotWaitForGrandchild` | A fake `gemini` starts the absolute `sleep 30` in the background with its output inherited, writes the process number of that `sleep` to a file, prints `0.68.0` and exits 0. `Scan` returns within 3 seconds with Gemini installed at `0.68.0`. At that moment the `sleep` process is still alive, which proves that it held the output. The test kills it in its cleanup. |
| `scan_test.go` | `TestScanBudgetCoversAllCandidates` | Fakes `cursor-agent` in a first folder, and `agent` in a second and a third folder, each write `.started` and then sleep. With `Timeout` 500 milliseconds, the first candidate uses the whole budget. Cursor is not installed with `ProblemTimedOut`, and neither `agent` fake has a `.started` file, which proves that no later candidate got a budget of its own. `Scan` returns within 3 seconds, which only guards against a hung test (logic-r2 12). |
| `scan_test.go` | `TestScanLaterCandidateGetsRemainingBudget` | A fake `cursor-agent` sleeps for 1.5 seconds and exits with 1. A fake `agent` in a later folder writes its `.started` file and then sleeps. With `Timeout` 2 seconds, the `.started` file of `agent` exists, and `Scan` returns within 3.5 seconds. A scanner that gave `agent` a fresh 2 seconds would take at least 4.5 seconds (logic-r2 12). |
| `scan_test.go` | `TestScanReportsFailureAndNoVersion` | A fake `claude` that exits with 1 gives `ProblemFailed`. A fake `gemini` that prints `0.68.0` and exits with 1 also gives `ProblemFailed` (critic-r1 8). A fake `codex` that prints `hello` gives `ProblemNoVersion`. All three are not installed. |
| `scan_test.go` | `TestCursorAgentNeedsCursorVersion` | With only a fake `agent` that prints `1.4.2`, Cursor is not installed with `ProblemNoVersion`. When a second folder, later in `SearchPath`, holds a fake `agent` that prints `2026.11.20-abc1234`, Cursor is installed from that second folder. |
| `scan_test.go` | `TestScanStopsOnCancel` | With a context cancelled before the call and four fakes that sleep, `Scan` returns within 2 seconds and reports none as installed. |

- **Acceptance:** `go test -race ./src/client/internal/harness/...` passes, and the run starts no program outside `t.TempDir()` except the absolute `sleep` that the fakes call.

**Tasks of WS-3:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 4.1 | Write the pure parsers and decoders, and the fixtures | `src/client/internal/sysinfo/parse.go`, `parseMeminfo` and `decodeDarwinLoadavg` (target), and the fixtures `testdata/meminfo` and `testdata/loadavg` (critic-r1 12). Pattern: `src/contracts/proto/lgtm/v1/clients.proto`, message `Client`, fields `cpu_percent` and `memory_percent`. |
| 4.2 | Write the readers for Linux and macOS, and the test for a Mac | `src/client/internal/sysinfo/sysinfo_linux.go` and `sysinfo_darwin.go`, `ReadFacts` and `ReadLoad`, and `sysinfo_darwin_test.go`, `TestReadLoadOnMacMatchesSystem` (target). Pattern: `parse.go` of task 4.1, and the field `MemTotal` of the Linux file `/proc/meminfo` (critic-r1 12). |
| 5.1 | Write the definitions of the four harnesses | `src/client/internal/harness/registry.go`, `Definitions` (target). Pattern: `packages/engine/src/harnesses/recipes.ts`, `installRecipes`. |
| 5.2 | Write the scan | `src/client/internal/harness/scan.go`, `Scan` (target). Pattern: `packages/engine/src/harnesses/programs.ts`, `makeHarnessPrograms`, inner function `inspect`. |
| 5.3 | Write the check of where a program lives and of the folders the child may search | `src/client/internal/harness/trust.go`, `trustedFolder` and `trustedProgram`, and `trust_linux.go`, `hasAccessList` (target). Pattern: `Open` of task 2.1, which applies the same rule to folders (threat-r1 06, architect-r2 5). |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The facts and the load are read on Linux. | `go test -run 'TestReadFactsOnLinux|TestReadLoadOnLinux' ./src/client/internal/sysinfo/` passes. |
| 2 | The values of macOS are decoded. | `go test -run 'TestDecodeDarwinLoadavg|TestDecodeDarwinUint64' ./src/client/internal/sysinfo/` passes, and `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./src/client/internal/sysinfo/` exits 0. |
| 3 | The scan finds fake harnesses and never touches the real ones. | `go test -run 'TestScanFindsFakePrograms|TestScanEmptySearchPathFindsNothing' ./src/client/internal/harness/` passes. |
| 4 | The scan runs no shell, runs no program from a place others can change, and leaks no secret. | `go test -run 'TestScanNeverUsesShell|TestScanRefusesExtraText|TestScanGivesMinimalEnvironment|TestScanChildPathIsCleaned|TestScanSkipsUntrustedFolder|TestScanSkipsWritableProgram|TestScanFollowsLinkToCheck|TestScanChildPathHoldsOnlyTrustedFolders|TestScanHelperInRefusedFolderIsNotFound|TestScanChecksEveryLinkStep|TestScanRefusesAccessList' ./src/client/internal/harness/` passes. |
| 5 | A program that never answers does not stop the scan. | `go test -run 'TestScanTimesOut|TestScanDoesNotWaitForGrandchild|TestScanBudgetCoversAllCandidates|TestScanLaterCandidateGetsRemainingBudget' ./src/client/internal/harness/` passes. |
| 6 | On a real Mac, the load and the memory match what the system reports. | On a Mac, `go test -run TestReadLoadOnMacMatchesSystem ./src/client/internal/sysinfo/` passes (critic-r1 9). Phase 5 of sprint 1 runs it. When no Mac is available, phase 5 logs the gap, and phase 5 of sprint 3 runs it again. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A harness prints a secret with its version, and the secret reaches the website. | high | Only a whole first line that matches the harness's pattern counts, proven by `TestScanRefusesExtraText` (threat-r1 10). The child gets no variable of the client, proven by `TestScanGivesMinimalEnvironment`. |
| Another user plants a program with a harness's name, or a helper that a harness looks up, in a folder they can write, and the scan runs it. | high | `trustedProgram` refuses a program, a link or a folder that another user can change, and the child searches only trusted folders, proven by `TestScanSkipsUntrustedFolder`, `TestScanChecksEveryLinkStep` and `TestScanHelperInRefusedFolderIsNotFound` (architect-r2 5). Access lists on macOS are a post-build check. |
| A program never ends, or a grandchild holds the output open. | medium | One time limit for all candidates of a harness, and `WaitDelay`, proven by `TestScanTimesOut`, `TestScanDoesNotWaitForGrandchild` and `TestScanBudgetCoversAllCandidates`. |
| Another program called `agent` is taken for Cursor. | medium | Cursor's pattern needs a version that starts with a date, proven by `TestCursorAgentNeedsCursorVersion`. |
| A harness changes the format of its version line, and shows as not installed. | medium | The result is visible as `ProblemNoVersion`, which is safe. A post-build check reads the real output of each harness. |
| The formula for the load differs from what a person expects on macOS. | low | Decision 23 records the assumption, and done-when row 6 checks it on a Mac. |
| A test runs a real harness on the developer's machine. | medium | `Scan` reads only the given search path, proven by `TestScanEmptySearchPathFindsNothing`. |

**Verify:** the tests of items 4 and 5. Device check: done-when row 6 on a Mac, in phase 5.

**Rollout detail:** step 2 of the rollout order.

**WS-4: Nothing the client must send or has done is lost in a crash**

This workstream builds the two records that the design's sections "Events come up" and "Commands go down" describe.
Since round 2, both live in one embedded store with transactions, `go.etcd.io/bbolt`, in the file `client.db`.
This follows a ruling of the orchestrator of 2026-11-26: two rounds in a row found blockers in the outbox built from one file for each event, an epoch file and a mark, so that mechanism was replaced and not patched again.
UNREVIEWED MECHANISM: every part of this workstream is new in round 2, and round 3 attacks it as a new design.
Item 6 is the store and the outbox in it. Item 7 is the receipts of commands in the same store.
The records hold opaque bytes, so they need no generated contract code. Plan `go-s3-client-link` puts the link's messages into them.

**Item 6. The store and the outbox**

- **Files:** `src/client/internal/link/store.go`, `src/client/internal/link/outbox.go`, `src/client/internal/link/store_test.go`, `src/client/internal/link/outbox_test.go`, `src/client/internal/link/store_crash_test.go`.
- **Needs first:** item 2, and `go.etcd.io/bbolt` in `go.mod`, which the result file asks of the Contract team. Runs at the same time as item 3.
- **Work area:** `backend`. **Complexity:** medium, because the store gives the atomic commit, and the item adds one creation step, the numbers, the limits and four small transactions, each with its own test.
- **Scope:** `client/backend`.
- **Design statements:** the section "Events come up" (numbered, on disk, within a limit, sent again from the first one not acknowledged), and C11.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Opening the store with `NoSync` set makes `TestStoreOpensWithSync` fail.
  2. Letting `Ack` accept a number above the sent mark makes `TestAckRefusesUnsent` fail (architect-r2 1, logic-r2 01).
  3. Letting `Reconcile` delete unsent events instead of moving them makes `TestReconcileNewStoreRenumbers` fail.
  4. Dropping the oldest event instead of refusing makes `TestAppendRefusesWhenFull` fail.
  5. Creating `client.db` in place instead of under a temporary name makes `TestOpenStoreCrashDuringCreation` fail.
  6. Going on after a failed commit makes `TestCommitErrorFailsStore` fail.
  7. Removing or rebuilding a store that does not open makes `TestOpenStoreRefusesDamagedFile` fail.
- **Public names that others build against:**
  - `type StoreOptions struct { MaxEvents int; MaxBytes int64; MaxEventBytes int; Keep time.Duration; MaxResultBytes int; Now func() time.Time }`, with `DefaultMaxEvents = 10000`, `DefaultMaxBytes = 64 << 20`, `DefaultMaxEventBytes = 1 << 20`, `DefaultReceiptKeep = 24 * time.Hour` and `DefaultMaxResultBytes = 1 << 20`. A number of 0 means its default, and a negative one is refused. `Now` has no default, and item 7 says why.
  - `type Store struct` and `func OpenStore(f *state.Folder, opts StoreOptions) (*Store, error)`. It claims `"store"` on the folder and uses the path that the claim returns.
  - `type Event struct { Seq uint64; Payload []byte }`.
  - `func (s *Store) Append(payload []byte) (uint64, error)`.
  - `func (s *Store) Read(from uint64, limit int) ([]Event, error)`.
  - `func (s *Store) MarkSent(upTo uint64) error`.
  - `func (s *Store) Ack(upTo uint64) error`.
  - `func (s *Store) Reconcile(serverLast uint64) (moved int, err error)`.
  - `func (s *Store) Acked() uint64`, `Sent() uint64`, `Next() uint64`, `Len() int`, `Bytes() int64`, and `Changed() <-chan struct{}`.
  - `func (s *Store) Close() error`.
  - `var ErrOutboxFull`, `ErrEventTooLarge`, `ErrSequenceExhausted`, `ErrAckAhead`, `ErrServerBehind`, `ErrStoreClosed`, `ErrStoreFailed`, `ErrStoreDamaged`, `ErrStoreUnsafe`, and `state.ErrStoreOpen`. Writer: items 6 and 7. Reader: plan `go-s3-client-link`.
  - Unexported, for tests only: `const maxSeq = 1 << 62`; `var commit = func(db *bolt.DB, fn func(*bolt.Tx) error) error { return db.Update(fn) }`, which every write goes through; and `var createHook func(step string)`, with the steps `"temp-created"`, `"temp-initialized"`, `"placed"` and `"dir-synced"`.
- **The format of the store.** Bucket `meta` holds `version`, the text `1`, and `next`, `acked`, `sent`, `count` and `bytes`, each an unsigned 64-bit big-endian value. Item 7 adds `pruned_through`. Bucket `events` maps an event number, as an unsigned 64-bit big-endian key, to its payload. Bucket `receipts` belongs to item 7. The numbers always keep `acked` at most `sent`, and `sent` below `next`. Every stored event has a number above `acked` and below `next`, and the numbers have no gap.
- **Behaviour of `OpenStore`:**
  - When `client.db` does not exist, `OpenStore` makes it whole before it gives it its name. It opens a new store `.tmp-<16 random hexadecimal characters>.db` with mode `0600`, creates the three buckets and the values of a new store in one transaction, closes it, links it to `client.db`, removes the temporary name, and syncs the state folder (architect-r2 7, logic-r2 05, logic2-r2 1). A stop before the link leaves only a temporary file, which the next `state.Open` removes. A link that finds `client.db` already there keeps that file.
  - `OpenStore` checks `client.db` with `os.Lstat` before it opens it. A link, a file that is not regular, a mode with a bit for group or others, or another owner gives `ErrStoreUnsafe`.
  - It opens the store with `bolt.Open`, mode `0600`, a time limit of 1 second for the store's own lock, and `NoSync` false. A time-out gives `state.ErrStoreOpen`. Any other failure gives `ErrStoreDamaged`, which wraps the cause.
  - In one read transaction it checks the version, the three buckets, and the rules of the format above, using only the first and the last event key. Any failure gives `ErrStoreDamaged`.
  - It then runs the step of item 7 that turns `started` receipts into `interrupted`.
  - It always syncs the state folder before it returns, so that the name `client.db` is durable even when an earlier open stopped after the link.
  - A store that does not open is never deleted, moved or rebuilt. The plan treats it as it treated a damaged outbox: the client refuses and deletes nothing, and the owner's choice of decision 48 applies.
- **Behaviour of the outbox:**
  - `Append` refuses a payload above `MaxEventBytes` with `ErrEventTooLarge` before any transaction. In one transaction it then reads `next`, `count` and `bytes`, refuses with `ErrOutboxFull` when the count would pass `MaxEvents` or the bytes would pass `MaxBytes`, refuses with `ErrSequenceExhausted` when `next` has reached `maxSeq`, stores the event under `next`, and writes `next`, `count` and `bytes` again. It returns the number only after the commit returned, and a commit of this store syncs the file before it returns. A number that was handed out and an event that is stored are then the same fact. It then signals `Changed()` without blocking.
  - `Read(from, limit)` returns, in order, at most `limit` events whose number is at least the larger of `from` and `acked` plus 1. It copies every payload inside its read transaction. A `limit` of 0 or less returns nothing.
  - `MarkSent(upTo)` records that the sender is about to write every event up to `upTo` to the stream. The sender calls it before it writes them. It does nothing when `upTo` is at most `sent`, returns `ErrAckAhead` when `upTo` is `next` or more, and otherwise writes `sent` in one transaction.
  - `Ack(upTo)` does nothing when `upTo` is at most `acked`. It returns `ErrAckAhead` and changes nothing when `upTo` is above `sent`, because the server cannot have applied an event that was never sent (architect-r2 1, logic-r2 01). Otherwise one transaction removes the events from `acked` plus 1 to `upTo`, and writes `acked`, `count` and `bytes`.
  - `Reconcile(serverLast)` takes the server's last applied number from `Welcome.last_event_number`, once on each connection and before any send. It works in one transaction and never deletes an event that was not sent.
    - When `serverLast` is below `acked`, it returns `ErrServerBehind` and changes nothing. A server that follows the contract raises its mark to `Hello.acked_event_number` first, so this means a fault of the server.
    - When `serverLast` is from `acked` to `sent`, it acts as `Ack(serverLast)`.
    - When `serverLast` is above `sent`, it removes the sent events, and it moves every unsent event to follow `serverLast` in the same order: the event `sent` plus `k` becomes `serverLast` plus `k`. It then writes `acked` and `sent` as `serverLast`, and `next` as one above the last moved number. It returns how many events it moved, so that the caller logs it. This is the case of a new store on a machine that the server already knows, and of a store restored from a backup (architect-r2 6, logic-r2 06, logic2-r2 2, logic2-r2 3). When the moved numbers would reach `maxSeq`, it returns `ErrSequenceExhausted` and changes nothing.
  - Any failure of a commit, other than the method's own refusals above, puts the store into a failed state. The call returns an error that wraps `ErrStoreFailed` and the cause, and every later call returns `ErrStoreFailed`. The store is never written again in this process. The fold "What plan go-s3-client-link must do" says that `run` then exits, so that the service starts it again and the store is opened afresh (architect-r2 10, logic-r2 02). UNREVIEWED MECHANISM: an event whose commit failed may or may not be stored, so the client stops instead of guessing.
  - All methods are safe to call at the same time.
  - `Close` closes the channel of `Changed()`, closes the store, and releases the claim. A second `Close` returns nil. After `Close`, every method that reads or writes returns `ErrStoreClosed`, and `Acked`, `Sent`, `Next`, `Len` and `Bytes` return their last values.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `store_test.go` | `TestOpenStoreCreatesPrivateFile` | On a new folder, `client.db` exists with mode exactly `0600`, `Next()` is 1, `Acked()` and `Sent()` are 0, and no name with the prefix `.tmp-` is left. |
| `store_test.go` | `TestStoreOpensWithSync` | The open store's `NoSync` is false, read through an unexported accessor of the test file. |
| `store_test.go` | `TestOpenStoreSyncsFolder` | With `state.SetSyncFolderForTest` recording, every `OpenStore` records a sync of the state folder. With the sync failing, `OpenStore` returns an error and no store. |
| `store_test.go` | `TestOpenStoreRefusesUnsafeFile` | A `client.db` that is a link to a valid store, and one of mode `0644`, each give `ErrStoreUnsafe`. |
| `store_test.go` | `TestOpenStoreRefusesDamagedFile` | A `client.db` of 64 kibibytes of random bytes, a valid store cut to half its length, a store whose `version` is `2`, and a store whose `acked` is above `sent` each give `ErrStoreDamaged`. After each refusal, the file's bytes are unchanged and no other file was made. |
| `store_test.go` | `TestSecondOpenStoreRefused` | A second `OpenStore` on the same `state.Folder` returns `ErrStoreOpen`. After `Close`, it succeeds. |
| `store_test.go` | `TestCommitErrorFailsStore` | With `commit` replaced by a function that commits and then returns an error, `Append(A)` returns an error that wraps `ErrStoreFailed`, and the next `Append` returns `ErrStoreFailed`. After `Close` and `OpenStore`, `A` is stored under its number, and the next append returns the number after it. With `commit` replaced by a function that returns an error without committing, after the reopen `A` is absent. In both cases no returned number ever names two payloads. |
| `outbox_test.go` | `TestAppendNumbersFromOne` | Three appends return 1, 2 and 3. `Read(0, 10)` returns the three payloads in order. |
| `outbox_test.go` | `TestReopenKeepsPending` | After three appends, `Close` and `OpenStore` again, `Read(0, 10)` returns the same three events, and the next append returns 4. |
| `outbox_test.go` | `TestReadFromFirstUnacked` | After five appends, `MarkSent(5)` and `Ack(2)`, `Read(0, 10)` returns 3, 4 and 5. `Read(4, 10)` returns 4 and 5. `Read(0, 2)` returns 3 and 4. |
| `outbox_test.go` | `TestAckIsCumulativeAndIdempotent` | After five appends, `MarkSent(5)` and `Ack(3)`, `Ack(2)` and `Ack(3)` return nil and change nothing. `Acked()` is 3. |
| `outbox_test.go` | `TestAckRefusesUnsent` | With events 1 to 3 stored and `MarkSent(1)`, `Ack(2)` and `Ack(math.MaxUint64)` each return `ErrAckAhead`, and events 1 to 3 stay stored (architect-r2 1, logic-r2 01). |
| `outbox_test.go` | `TestMarkSentRefusesUnstored` | With events 1 and 2 stored, `MarkSent(3)` returns `ErrAckAhead`, and `Sent()` stays 0. |
| `outbox_test.go` | `TestReconcileCases` | Each case starts from a fresh copy with `acked` 2, `sent` 4 and events 3 to 5 stored. `Reconcile(1)` gives `ErrServerBehind` and changes nothing. `Reconcile(2)` changes nothing. `Reconcile(3)` leaves 4 and 5. `Reconcile(4)` leaves 5. `Reconcile(5)` removes 3 and 4, moves 5 to 6, returns 1 moved, and leaves `Next()` at 7 (logic2-r2 7). `Reconcile(maxSeq)` gives `ErrSequenceExhausted` and changes nothing. |
| `outbox_test.go` | `TestReconcileNewStoreRenumbers` | On a new store, two appends return 1 and 2. `Reconcile(50)` returns 2 moved. `Read(0, 10)` returns the same two payloads under 51 and 52, and the next append returns 53. On another new store, `Reconcile(0)` changes nothing. |
| `outbox_test.go` | `TestReconcileAfterRestoreKeepsUnsent` | With `acked` 0, `sent` 3 and events 1 to 5 stored, `Reconcile(10)` removes 1 to 3, and events 4 and 5 are stored under 11 and 12 with their payloads. |
| `outbox_test.go` | `TestOutboxNeverReusesNumbers` | After five appends, `MarkSent(5)`, `Ack(5)`, a reopen, and one append, the number is 6. |
| `outbox_test.go` | `TestAppendRefusesWhenFull` | With `MaxEvents` 3, the fourth append returns `ErrOutboxFull`, `Len()` stays 3, and events 1 to 3 are unchanged. After `MarkSent(1)` and `Ack(1)`, an append succeeds and returns 4. With `MaxBytes` 10, appends of 6 bytes and then 5 bytes give success and then `ErrOutboxFull`. |
| `outbox_test.go` | `TestAppendRefusesLargeEvent` | With `MaxEventBytes` 8, a payload of 9 bytes gives `ErrEventTooLarge`, and `Next()` is unchanged. |
| `outbox_test.go` | `TestAppendRefusesAtSequenceCeiling` | A store whose `next` the test sets to `maxSeq` gives `ErrSequenceExhausted` on the next append, and nothing changes. |
| `outbox_test.go` | `TestChangedSignalsAppend` | After an append, a receive from `Changed()` succeeds within 1 second. |
| `outbox_test.go` | `TestConcurrentAppendsAreUnique` | 8 goroutines append 50 events each. The 400 numbers are 1 to 400 with no repeat, and `Read(0, 1000)` returns 400 events in order. |
| `outbox_test.go` | `TestReadDuringAckNeverFails` | For 1,000 rounds, one goroutine appends, marks sent and acknowledges while another reads. No `Read` returns an error. |
| `outbox_test.go` | `TestCloseWakesChangedAndKeepsSnapshot` | A goroutine waiting on `Changed()` returns within 1 second of `Close`. The five accessors return their values from before `Close`. `Append`, `Read`, `MarkSent`, `Ack` and `Reconcile` return `ErrStoreClosed`. A second `Close` returns nil. |
| `store_crash_test.go` | `TestOpenStoreCrashDuringCreation` | For each of the four steps of creation, a child opens a new store and exits with 86 at that step. The parent asserts that the child exited with 86. After `state.Open` and `OpenStore` in the parent, the store opens, `Next()` is 1, and no `.tmp-` name is left (architect-r2 7). |
| `store_crash_test.go` | `TestStoreKillKeepsEveryNumberedEvent` | A child appends events in a loop. The payload of event `n` is the text `event-<n>` repeated up to a size between 1 byte and 64 kibibytes that depends on `n`. After each append returns, it prints `appended <n>`, and at every tenth number it calls `MarkSent` and `Ack` for it. The parent waits for the child's first `appended` line, then a random 0 to 30 milliseconds, then kills it with `SIGKILL`. It does this 30 times in a row, from a fixed seed that the test logs, and the 30 runs must print at least 100 numbers in all. After each kill, `OpenStore` succeeds. Every number that the child printed is either at most `Acked()` or stored with its exact payload. The stored numbers run without a gap from `Acked()` plus 1. The first number the next child prints is larger than every number printed before. |
| `store_crash_test.go` | `TestStoreKeepsOrderAndOnceAcrossRestart` | The parent plays the server, with a list of applied events and a mark. In 20 rounds, a child appends 1 to 20 events and is killed at a random moment. The parent then opens the store, calls `Reconcile` with its mark, calls `MarkSent`, applies every event whose number is its mark plus 1, and acknowledges. At the end, the applied list holds every number that a child printed exactly once, in rising order, each with its exact payload. |

The child processes of this item run the helper test `TestHelperStoreProcess` in `store_crash_test.go`, with a mode for each test. Item 7 has its own helper, `TestHelperReceiptsProcess`.

- **Acceptance:** `go test -race -run 'TestOpenStore|TestStore|TestSecondOpenStore|TestCommitError|TestAppend|TestReopen|TestRead|TestAck|TestMarkSent|TestReconcile|TestOutbox|TestChanged|TestConcurrentAppends|TestClose' ./src/client/internal/link/` passes. The break reviewer's report names the seven mutations and the test that went red for each.

**Item 7. Receipts of commands**

- **Files:** `src/client/internal/link/receipts.go`, `src/client/internal/link/receipts_test.go`, `src/client/internal/link/receipts_crash_test.go`.
- **Needs first:** item 6.
- **Work area:** `backend`. **Complexity:** medium, because it is one record with six states in a bucket of item 6, and each rule is one transaction with its own test.
- **Scope:** `client/backend`.
- **Design statements:** the section "Commands go down" (a command that arrives twice runs once and the result is sent again, and a command past its expiry is not run), R6 and R9.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Writing the `started` receipt after `Begin` returns makes `TestBeginWritesStartedBeforeReturn` and the count of effects in `TestReceiptsKillNeverRunsTwice` fail.
  2. Leaving `started` receipts alone at open makes `TestOpenMarksStartedAsInterrupted` fail.
  3. Skipping the check of the expiry makes `TestBeginRefusesExpired` fail.
  4. Skipping the check of the identifier makes `TestBeginRejectsBadIDs` fail.
  5. Pruning a damaged receipt makes `TestDamagedReceiptIsNeverPruned` fail.
  6. Writing `pruned_through` in a separate commit from the removal makes `TestPruneRaisesWatermarkInSameCommit` fail (architect-r2 4, logic-r2 04).
  7. Running a higher attempt of a command that ended makes `TestReplayTruthTable` fail.
  8. Keeping the first attempt while a newer one arrives during the run makes `TestArrivalWhileRunningRaisesAttempt` fail (architect-r2 11, logic-r2 09).
- **Public names that others build against:**
  - `type Command struct { ID string; Attempt uint32; ExpiresAt time.Time }`.
  - `type ReceiptState string` with `ReceiptStarted = "started"`, `ReceiptDone = "done"`, `ReceiptFailed = "failed"`, `ReceiptUnsupported = "unsupported"`, `ReceiptInterrupted = "interrupted"`, `ReceiptExpired = "expired"`. Plan `go-s3-client-link` sends them as `COMMAND_OUTCOME_DONE`, `FAILED`, `UNSUPPORTED`, `INTERRUPTED` and `EXPIRED` of the draft contract (architect-r2 11).
  - `type Receipt struct { CommandID string; Attempt uint32; LatestAttempt uint32; ExpiresAt time.Time; State ReceiptState; StartedAt time.Time; FinishedAt time.Time; Result []byte; Note string; Pinned bool }`. `LatestAttempt` is the highest attempt that arrived while the command ran. `Note` is empty, `result too large` or `damaged receipt`.
  - `type Decision int` with `DecisionRun Decision = 1`, `DecisionReplay Decision = 2`, `DecisionRunning Decision = 3`.
  - Methods on the `Store` of item 6: `func (s *Store) Begin(c Command) (Decision, Receipt, error)`, `func (s *Store) Finish(id string, state ReceiptState, result []byte) (Receipt, error)`, `func (s *Store) Get(id string) (Receipt, bool, error)` and `func (s *Store) Prune() (int, error)`.
  - `var ErrInvalidCommand`, `ErrNotStarted`, `ErrResultTooLarge`, `ErrNoClock`. Writer: item 7. Reader: plan `go-s3-client-link`.
- **Format of one receipt:** the value under the command's identifier in bucket `receipts` is a JSON object with `version` (always 1), `command_id`, `attempt`, `latest_attempt`, `expires_at`, `state`, `started_at`, `finished_at`, `result`, `note` and `pinned`. The times are RFC 3339 (Request for Comments 3339) text with nanoseconds, in UTC (Coordinated Universal Time). `finished_at` is left out while it is zero. `result` is the base64 of the bytes, and it is left out when empty. `note` and `pinned` are left out when empty or false. The value `pruned_through` in bucket `meta` is the latest expiry of any receipt that `Prune` removed, as RFC 3339 text, and it is absent until the first removal.
- **The clock:** `StoreOptions.Now` is the clock for expiry and pruning, and `Begin` and `Prune` return `ErrNoClock` when it is not set. Plan `go-s3-client-link` passes the count of the server's time that the draft contract defines: `Welcome.server_time` plus the time the monotonic clock measured since the client sent `Proof` (logic2-r2 4). A change of the client's own clock then reaches no decision. Round 2 dropped the file `receipts/clock` and its high-water mark, which the ruling made unneeded (logic2-r2 8).
- **Behaviour:**
  - `Begin` refuses with `ErrInvalidCommand` an `ID` that is not 36 characters of lowercase hexadecimal digits and hyphens in the canonical form of a universally unique identifier (UUID), an `Attempt` of 0, and a zero `ExpiresAt`. It writes nothing then.
  - `Begin` decides in one transaction.
    - A receipt in the state `started` gives `DecisionRunning`. When the arriving attempt is higher than `LatestAttempt`, the transaction first raises `LatestAttempt` to it (architect-r2 11, logic-r2 09).
    - A receipt in any other state gives `DecisionReplay` and the receipt, whatever the attempt and the expiry of the arriving command, and writes nothing (logic2-r2 5). The draft contract keeps the expiry of a command the same on every attempt, so round 2 dropped the rewrite of the stored expiry.
    - With no receipt, a command whose `ExpiresAt` is at or before `pruned_through` gets a receipt in the state `expired` and `DecisionReplay`. Its receipt may have been pruned, so it never runs (architect-r2 4, logic-r2 04).
    - With no receipt, a command for which `Now()` is at or after `ExpiresAt` gets a receipt in the state `expired` and `DecisionReplay`.
    - Otherwise `Begin` writes a receipt in the state `started`, with `Attempt` and `LatestAttempt` both the arriving attempt, and returns `DecisionRun` only after the commit returned.
  - `Finish` takes `ReceiptDone`, `ReceiptFailed` or `ReceiptUnsupported`, and refuses with `ErrNotStarted` an identifier whose receipt is not `started`. A result above `MaxResultBytes` is recorded as `failed` with an empty result and the note `result too large`, and `Finish` returns that receipt with `ErrResultTooLarge`. Otherwise it writes the receipt in the given state with the result and the time, and returns it after the commit. The returned receipt carries `LatestAttempt`, under which plan `go-s3-client-link` sends the result.
  - A failed commit in `Begin`, `Finish` or `Prune` puts the store into the failed state of item 6. A command whose `Begin` failed is not run. A command whose `Finish` failed is recorded as `interrupted` when the store is next opened, and the server gets that outcome (logic-r2 09).
  - The step that `OpenStore` runs: in one transaction, every value in bucket `receipts` that fails these checks becomes `interrupted`, pinned, with the note `damaged receipt`. The checks are that the JSON decodes with no unknown field, `version` is 1, `command_id` equals the key, `state` is one of the six, `attempt` and `latest_attempt` are at least 1, `expires_at` and `started_at` are not zero, and `finished_at` is not zero for the five ended states. In the same transaction, every valid receipt in the state `started` becomes `interrupted` with an empty result.
  - `Prune` works in one transaction. It removes every receipt that is not `started` and not pinned, and whose latest time of `ExpiresAt`, `StartedAt` and `FinishedAt` is more than `Keep` before `Now()`. In the same transaction it raises `pruned_through` to the latest `ExpiresAt` it removed. It returns the number removed. Plan `go-s3-client-link` calls it only after `Welcome`, when the count of the server's time exists.
  - A replayed result is the result of the attempt that ran it. Plan `go-s3-client-link` labels the reply with the attempt of the command that just arrived.
  - All methods are safe to call at the same time.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `receipts_test.go` | `TestBeginRunsNewCommand` | A new valid command gives `DecisionRun`, and `Get` gives a receipt in the state `started`. |
| `receipts_test.go` | `TestBeginWritesStartedBeforeReturn` | With `commit` replaced by a function that commits and then waits for the test, a `Begin` started in a goroutine has not returned after 200 milliseconds. After the release, it returns `DecisionRun`. |
| `receipts_test.go` | `TestSecondArrivalReplaysResult` | After `Begin` and `Finish(id, ReceiptDone, "ok")`, a second `Begin` with attempt 2 gives `DecisionReplay`, the state `done`, the result `ok` and the attempt 1. |
| `receipts_test.go` | `TestArrivalWhileRunningRaisesAttempt` | A second `Begin` with attempt 2 before `Finish` gives `DecisionRunning`. A third with attempt 1 changes nothing. `Finish` then returns `LatestAttempt` 2 (architect-r2 11, logic-r2 09). |
| `receipts_test.go` | `TestReplayWritesNothing` | With `commit` replaced by a counter, a replay of a `done` receipt with a later expiry makes no commit (logic2-r2 5). |
| `receipts_test.go` | `TestReplayTruthTable` | For the stored states none, `started`, `done`, `failed`, `unsupported`, `interrupted` and `expired`, crossed with the same and a higher attempt, and with the expiry before and after `Now()`: none with a later expiry gives `DecisionRun`; none with a past expiry gives `DecisionReplay` with `expired`; `started` gives `DecisionRunning`; every ended state gives `DecisionReplay` with that state. No ended state ever gives `DecisionRun`. |
| `receipts_test.go` | `TestBeginRefusesExpired` | With `Now` fixed at noon and `ExpiresAt` one second before noon, `Begin` gives `DecisionReplay` with the state `expired`, and a second `Begin` gives the same. |
| `receipts_test.go` | `TestBeginExpiresAtTheInstant` | With `Now` equal to `ExpiresAt`, `Begin` gives `DecisionReplay` with `expired`. |
| `receipts_test.go` | `TestBeginNeedsClock` | With `Now` not set, `Begin` and `Prune` return `ErrNoClock` and write nothing. |
| `receipts_test.go` | `TestFinishedSurvivesReopen` | After `Finish`, `Close` and `OpenStore`, `Begin` gives `DecisionReplay` with the same result. |
| `receipts_test.go` | `TestOpenMarksStartedAsInterrupted` | After `Begin` with no `Finish`, `Close` and `OpenStore`, `Get` gives the state `interrupted`, and `Begin` gives `DecisionReplay`. `Finish` gives `ErrNotStarted`. |
| `receipts_test.go` | `TestDamagedReceiptIsNeverPruned` | A value `{` stored by the test under a valid identifier becomes `interrupted`, pinned, with the note `damaged receipt`. With `Now` ten years later, `Prune` keeps it. After two more reopens, `Begin` of that identifier with an expiry in the future gives `DecisionReplay` with `interrupted` each time. |
| `receipts_test.go` | `TestValidJSONWithBadContentIsDamaged` | Values with no `state`, with `version` 2, with a `command_id` that differs from the key, with the state `weird`, and with a zero `started_at` each become `interrupted` and pinned. |
| `receipts_test.go` | `TestPruneRaisesWatermarkInSameCommit` | With `Keep` 10 seconds, a command with the expiry T plus 15 seconds runs and finishes. With `Now` at T plus 30 seconds, `Prune` removes it, and `pruned_through` is T plus 15 seconds. After a reopen with `Now` at T plus 5 seconds, a `Begin` of the same identifier gives `DecisionReplay` with `expired`. With `commit` failing during a second such prune, the receipt is still there after the reopen, and `pruned_through` is unchanged (architect-r2 4, logic-r2 04). |
| `receipts_test.go` | `TestClockForwardJumpHeals` | With `Now` one day ahead, one command runs and `Prune` runs. With `Now` back at the right time, a new command with an expiry in one hour gives `DecisionRun` (logic2-r2 4). |
| `receipts_test.go` | `TestBeginRejectsBadIDs` | `../machine.key`, `a/b`, an uppercase UUID, a UUID without hyphens, and the empty text each give `ErrInvalidCommand`. An attempt of 0 and a zero expiry give the same. Nothing is written. |
| `receipts_test.go` | `TestFinishTooLargeIsTerminal` | With `MaxResultBytes` 4, `Finish` with 5 bytes returns `ErrResultTooLarge` and a receipt in the state `failed` with the note `result too large`. A later `Begin` gives `DecisionReplay` with `failed`. |
| `receipts_test.go` | `TestFinishUnsupported` | `Finish(id, ReceiptUnsupported, nil)` stores `unsupported`, and a later `Begin` replays it (architect-r2 11). |
| `receipts_test.go` | `TestBeginAfterCommitError` | With `commit` failing, `Begin` of a new command returns an error that wraps `ErrStoreFailed`, and not `DecisionRun`. After a reopen with a working commit, `Begin` of that identifier gives `DecisionRun` when nothing was stored, or `DecisionReplay` with `interrupted` when the failed commit had been stored. |
| `receipts_test.go` | `TestPruneRemovesOldReceipts` | With `Keep` one hour, a done receipt whose times are all two hours before `Now()` is removed, and one whose expiry is in 30 minutes stays. A `started` receipt and a pinned receipt are never removed. `Prune` returns 1. |
| `receipts_test.go` | `TestReceiptFormat` | The stored value decodes as JSON with exactly the fields of the format above, `version` is 1, and the times end in `Z`. |
| `receipts_test.go` | `TestConcurrentBeginRunsOnce` | 16 goroutines call `Begin` with the same command. Exactly one gets `DecisionRun`. |
| `receipts_crash_test.go` | `TestReceiptsKillNeverRunsTwice` | A child loops over identifiers made from a counter. For each, it calls `Begin`. After `DecisionRun` it appends the identifier as one line to the file `effects` with a sync, prints `begun <id>`, calls `Finish` with `ReceiptDone` and the result `result-<id>`, and prints `finished <id>` after it returns. The parent kills the child with `SIGKILL` after its first `begun` line and a random 0 to 30 milliseconds, 30 times in a row, from a fixed seed that the test logs. After each kill, no identifier appears twice in `effects` across all the runs. Every identifier printed as `finished` has the state `done` and the exact result. Every identifier in `effects` has a receipt in the state `done` or `interrupted`. `Begin` of each identifier in `effects` never gives `DecisionRun`. |

- **Acceptance:** `go test -race -run 'TestBegin|TestSecondArrival|TestArrival|TestReplay|TestFinish|TestOpenMarks|TestDamaged|TestValidJSON|TestPrune|TestClock|TestReceipt|TestConcurrentBegin' ./src/client/internal/link/` passes. The break reviewer's report names the eight mutations and the test that went red for each.

**Tasks of WS-4:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 6.1 | Write the creation, the opening and the failed state of the store | `src/client/internal/link/store.go`, `OpenStore` and `Close` (target). Pattern: `CreateExclusive` of task 2.2, which also links a finished file to its name, and the function `Open` of `go.etcd.io/bbolt` (`db.go`). |
| 6.2 | Write the outbox in the store | `src/client/internal/link/outbox.go`, `Append`, `MarkSent`, `Ack` and `Reconcile` (target). Pattern: `floorplan/plan.json`, entry "Events waiting to be sent", and the method `NextSequence` of a bucket of `go.etcd.io/bbolt` (`bucket.go`). |
| 7.1 | Write the receipts in the store | `src/client/internal/link/receipts.go`, `Begin`, `Finish` and `Prune` (target). Pattern: the design's section "Commands go down", and the comment at the top of `.plans/plans/go-s0-contract/draft/commands.proto`. |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Every event that got a number survives a kill, whole and in order, and arrives once across a restart. | `go test -run 'TestStoreKillKeepsEveryNumberedEvent|TestStoreKeepsOrderAndOnceAcrossRestart' ./src/client/internal/link/` passes. |
| 2 | A store that stopped while it was made opens as new, and a store that does not open is left untouched. | `go test -run 'TestOpenStoreCrashDuringCreation|TestOpenStoreRefusesDamagedFile|TestOpenStoreRefusesUnsafeFile' ./src/client/internal/link/` passes. |
| 3 | A number is never given to two events, and a failed commit stops the store. | `go test -run 'TestOutboxNeverReusesNumbers|TestConcurrentAppendsAreUnique|TestCommitErrorFailsStore|TestStoreOpensWithSync' ./src/client/internal/link/` passes. |
| 4 | An acknowledgement or a number from the server never deletes an event that was not sent. | `go test -run 'TestAckRefusesUnsent|TestMarkSentRefusesUnstored|TestReconcileCases|TestReconcileNewStoreRenumbers|TestReconcileAfterRestoreKeepsUnsent' ./src/client/internal/link/` passes. |
| 5 | A full outbox refuses and loses nothing. | `go test -run 'TestAppendRefusesWhenFull|TestAppendRefusesAtSequenceCeiling' ./src/client/internal/link/` passes. |
| 6 | A command that arrives twice runs once, and its result goes out under the newest attempt. | `go test -run 'TestSecondArrivalReplaysResult|TestConcurrentBeginRunsOnce|TestReplayTruthTable|TestArrivalWhileRunningRaisesAttempt' ./src/client/internal/link/` passes. |
| 7 | No command runs twice across a kill, a damaged receipt, a failed commit or a prune. | `go test -run 'TestReceiptsKillNeverRunsTwice|TestOpenMarksStartedAsInterrupted|TestDamagedReceiptIsNeverPruned|TestValidJSONWithBadContentIsDamaged|TestBeginAfterCommitError|TestPruneRaisesWatermarkInSameCommit' ./src/client/internal/link/` passes. |
| 8 | A command past its expiry is not run, and a bad identifier is never stored. | `go test -run 'TestBeginRefusesExpired|TestBeginExpiresAtTheInstant|TestBeginRejectsBadIDs|TestBeginNeedsClock' ./src/client/internal/link/` passes. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| An event is sent before it is on disk, and a crash gives its number to another event. | high | The number is taken and the event stored in one commit that syncs before `Append` returns, proven by `TestStoreKillKeepsEveryNumberedEvent` and `TestStoreOpensWithSync`. |
| An acknowledgement or a number from the server deletes events that were never sent. | high | `Ack` stops at the sent mark, and `Reconcile` moves unsent events instead of deleting them, proven by `TestAckRefusesUnsent` and `TestReconcileNewStoreRenumbers` (architect-r2 1, logic-r2 01). |
| A commit fails after the store wrote its pages, and the client cannot tell whether the event is stored. | high | The store stops, `run` exits, and the next start reads the disk, proven by `TestCommitErrorFailsStore`. A real failure of the disk is a post-build check. UNREVIEWED MECHANISM. |
| A stop while the store is first made leaves a file that never opens. | medium | The store is made under a temporary name and linked when whole, proven by `TestOpenStoreCrashDuringCreation` (architect-r2 7). |
| A damaged store keeps the machine off the website. | medium | The client refuses and deletes nothing. What to do next is decision 48, for the owner. |
| A command runs twice after a crash. | high | The `started` receipt is committed before the command runs, and an open turns it into `interrupted`, proven by `TestReceiptsKillNeverRunsTwice`. |
| A receipt is pruned, and a copy of its command arrives later and runs. | high | `pruned_through` rises in the same commit as the removal, and a command at or before it never runs, proven by `TestPruneRaisesWatermarkInSameCommit` (architect-r2 4, logic-r2 04). |
| An identifier from the server holds a path. | medium | Only the canonical form of a UUID is accepted, proven by `TestBeginRejectsBadIDs`. The store keys by the text, so no file is named by it. |
| The store is a new dependency that the Contract team must add. | medium | The result file asks for it. Version 1.4.3 builds without a C compiler for the four targets, as `scouting.md` records. |
| A long outage fills the outbox. | medium | The outbox refuses, and the producer keeps its report or makes a newer one, as the fold "What plan go-s3-client-link must do" says. |

**Verify:** the tests of items 6 and 7, all under `-race`. Device check: none. The kill tests run on Linux, and phase 5 runs them on a Mac when one is available. The fold "Post-build checks" covers a real loss of power and a failing disk.

**Rollout detail:** steps 3 and 4 of the rollout order.

</details>

<details><summary><b>What plan go-s3-client-link must do</b></summary>

This plan builds pieces that are safe only when their caller keeps a few rules.
Plan `go-s3-client-link` is that caller, so its planner reads this list before freezing the plan.
Each rule names the finding that it answers. Round 2 rewrote the rules on events and commands for the store of item 6. UNREVIEWED MECHANISM.

**Events.**

1. A producer of an event keeps its event when `Append` refuses it with `ErrOutboxFull`, and tries again later, or it skips the event only when a newer event replaces it, as a newer load report replaces an older one (architect-r1 6).
2. A scan of the harnesses counts as finished only when its report is stored. A scan whose report was refused runs again.
3. One sender per machine talks to the store. On each connection it sends `Hello.acked_event_number` from `Acked()` (logic2-r2 2). When `Welcome` arrives, it calls `Reconcile` once with `Welcome.last_event_number`, before it sends any event, and logs the number of moved events when it is not 0. It then reads pending events in order, calls `MarkSent` for the highest number of a batch before it writes the batch to the stream, and calls `Ack` with `EventAck.through` only for an acknowledgement that came on the current stream (architect-r2 1, logic-r2 01). `ErrAckAhead` closes the stream, and the next connection reconciles.
4. The payload of an event is a versioned report body that carries the time the producer made it. A repeat of the same content under a new number is allowed. The server applies each kind of report so that the one with the latest producer time wins, and an older copy changes nothing. The result file asks the Server team for this rule (architect-r2 10, logic2-r1 9).
5. Any error that wraps `ErrStoreFailed` makes `run` log one sentence and exit with status 1, so that the service starts it again. On each start, the producers make their reports anew: the facts on the connection and a scan of the harnesses. A report whose commit failed is then either stored and sent, or made again, and rule 4 makes both safe (architect-r2 10, logic-r2 02).
6. `ErrServerBehind` closes the stream, logs one sentence, and retries with the growing wait of the sender, at most 15 seconds, as the design's rule for reconnection says (logic2-r2 3).
7. When `OpenStore` returns `ErrStoreDamaged` or `ErrStoreUnsafe`, `run` exits with one sentence that names `client.db`, until the owner rules on decision 48.
8. While the stream is down, the client appends a load report every 10 seconds as an event, in place of the heartbeat it cannot send. After each `Append` returns, it logs the line `link_event_buffered` with the field `sequence`, which check C11 of the user-flow test `deploy/test/checks.sh` reads. A load report that the full store refuses is skipped, because a newer one replaces it (architect-r2 9). The planner of `go-s3-client-link` may choose another trigger, but check C11 needs at least three events within 50 seconds while the server is stopped.

**Commands.**

9. A command runs only after `Begin` returns `DecisionRun`. Any error from `Begin` means the command does not run.
10. The client calls `Finish` for every command it ran, before it restarts or exits for any reason it controls. The command `Restart` calls `Finish` first and exits after it (logic-r1 8).
11. An arrival that gets `DecisionRunning` sends nothing at once. When `Finish` returns, the client sends the result on the current stream under `Receipt.LatestAttempt`, which is the newest attempt that arrived (architect-r2 11, logic-r2 09).
12. A replay sends the stored state under the attempt of the command that just arrived: `done` as `COMMAND_OUTCOME_DONE`, `failed` as `FAILED`, `unsupported` as `UNSUPPORTED`, `interrupted` as `INTERRUPTED` and `expired` as `EXPIRED`.
13. When `Finish` returns `ErrResultTooLarge`, the client sends the receipt it returned, which is `failed`.
14. A command whose action the client does not know goes through `Begin`, and then `Finish` with `ReceiptUnsupported` (architect-r2 11).
15. `StoreOptions.Now` is the count of the server's time from the draft contract. `Prune` runs only after `Welcome`.

**Joining and the state folder.**

16. `join` opens the state folder with `state.Open` and holds it until it ends. `run` does the same (threat-r1 04).
17. `join` refuses a folder that already holds `identity.json`. A new enrollment needs a new folder (logic2-r1 2).
18. `join` calls `enroll.Load` first. Only on `ErrKeyMissing` does it call `enroll.Create`. On `ErrPlacedNotSynced` from `Create`, it calls `state.SyncDir` on the folder, and it calls the server only after that sync succeeds. It never calls the server with a key whose placement is uncertain (architect-r2 8, logic-r2 08).
19. `join` writes `identity.json` with `state.WriteAtomic` last, only after the server confirmed the join. On `ErrPlacedNotSynced`, it calls `state.SyncDir` once more, and it reports success only after a sync succeeded. When the sync fails, it exits with one sentence that says the join was confirmed but not saved, and the next `join` or `run` syncs the folder before it trusts the file.
20. A key without `identity.json` means a join that did not finish. The next `join` reuses the key with the same token while the token is valid, which the draft contract allows. When the token has expired, the owner removes the half-joined machine on the website, removes the state folder, makes a new token, and runs `join` again. Plan `go-s3-client-link` prints this procedure in the sentence of the refusal (architect-r2 8, logic-r2 08).
21. `run` refuses to start when `identity.json` is missing or not valid, with one sentence that says to run `join`. It syncs the folder before it uses the file.
22. `run` logs the resolved state folder and the resolved folder for clones once at start (logic2-r1 12).
23. The report of the facts sends `Settings.RepositoriesFolder` as `Facts.workspace`, with the home folder written as `~`, and skips the field when it is longer than the contract's 256 bytes (architect-r2 12).

**The installer.**

24. Plan `go-s3-stack-client` sets no variable for the state folder. Its `join`, the service, and a person at a shell then all use the default rule of the layout and reach the same folder and the same clones (architect-r2 12, logic-r2 11). This withdraws the rule of round 1 that set `LGTM_CLIENT_STATE_DIR` in the service file.

</details>

<details><summary><b>Post-build checks</b></summary>

These checks need the built code, so they run in phase 5 of the build loop, or in sprint 3 where a row says so.
Each row gives the scenario and the result that must be seen.

| # | Check | Scenario | Expected result | From |
|---|---|---|---|---|
| 1 | A real loss of power on Linux | On a test machine with the client's state folder on its own disk, a loop appends events and runs commands. The machine's power is cut 10 times at random moments. | After each restart, `OpenStore` succeeds or names `client.db`. Every event whose number was printed is stored or acknowledged, and no command appears twice in the file of effects. | architect-r1 1, critic-r1 2, logic-r1 4 |
| 2 | A real loss of power on macOS | The same as check 1, on a Mac with the folder on its internal disk. | The same as check 1. A gap is logged when no Mac is available. | critic-r1 2 |
| 3 | The order after a reconnection | In sprint 3, the server is stopped while the client appends 50 events, and then started again. | The server applies all 50 events once and in order, and `Reconcile` is called once on the new connection. | architect-r1 3 |
| 4 | A join that stops half-way | In sprint 3, the client is killed after `machine.key` is written and before `identity.json`, once before the server confirmed and once after. `join` then runs again with the same token. | The second `join` reuses the key, and the server shows one machine, not two. After the token expired, the procedure of rule 20 leaves one machine. | architect-r1 7, architect-r2 8, logic-r2 08 |
| 5 | The owner checks as the superuser | A person runs the tests of items 2, 3 and 6 as the superuser, with a key file, a store file and a state folder owned by another user. | `Open` returns `ErrFolderNotOwned`, `Load` returns `ErrKeyNotOwned`, and `OpenStore` returns `ErrStoreUnsafe`. | threat-r1 04, threat-r1 11 |
| 6 | The scan under load | The scan runs while the machine runs a build that keeps every core busy. | `Scan` returns within `Timeout` plus `WaitDelay` plus 1 second, and each installed harness is found. | architect-r1 9 |
| 7 | The real version lines | On a machine with each of the four harnesses installed, a person runs `Scan`. | Each installed harness is found with its version. A harness whose line does not match is recorded, and its pattern is fixed. | threat-r1 10 |
| 8 | The report of the harnesses on the wire | In sprint 3, the harness report reaches the server. | The report carries only the harness name and the version. It carries no path and no problem text. | architect-r1 9 |
| 9 | The count of effects of a command | In sprint 3, a command with a visible effect is sent twice, and the client is killed once while it runs. | The effect happens at most once, and the second arrival gets the stored outcome. | logic-r1 2, critic-r1 4 |
| 10 | Samples from macOS | On a Mac, a person saves the raw bytes of `vm.loadavg`, `hw.memsize` and `kern.memorystatus_level` as fixtures for item 4. | The decoders of item 4 give the values that `/usr/sbin/sysctl -n` prints for the same names. | critic-r1 9 |
| 11 | A disk that fails a sync | On Linux, the state folder sits on a device-mapper target that is switched to return errors while the client appends events. The device is then switched back, and the service starts the client again. | The client exits with one sentence about `ErrStoreFailed`. After the restart, no number names two payloads, and the server applies every stored event once. | architect-r2 10, logic-r2 02 |
| 12 | Access lists on macOS | On a Mac, a person adds an access list that lets another user write to a folder on the search path, with `chmod +a`. | The result is recorded. The scan of round 2 does not check access lists on macOS, so the gap is logged if the harness is still found. | architect-r2 5 |
| 13 | Common installs of the harnesses | On a Mac with the Homebrew prefix `/usr/local`, and on Linux with a global install of a package manager, a person installs a harness and runs `Scan`. | The result is recorded for the owner's choice in decision 56. | logic2-r2 6 |
| 14 | A new attempt during a running command | In sprint 3, the stream breaks while a scan runs, and the server sends attempt 2 on the new stream. | The result reaches the server under attempt 2, and the command ran once. | architect-r2 11, logic-r2 09 |
| 15 | Events while the server is stopped | In sprint 3, check C11 of `deploy/test/checks.sh` runs. | The client logs at least three `link_event_buffered` lines while the server is stopped, and the server applies a later number after it is back. | architect-r2 9 |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe:

1. The six work items of the brief (`plan.md` of 2026-11-25).
2. The four points the brief asks the planner to settle.
3. The four points of the task file `.foundry/go-rewrite/tasks/plan-go-s1-client-local.md` under "Specific to this plan".
4. The statements of the design that this plan builds on, as listed in `scouting.md`: D2, D7, D8, D16, D19, R1, R6, R9, R10, C7, C10, C11, and the sections "Joining" step 3, "Commands go down", "Events come up", "What travels in wave 1", "Knowing whether a machine is there" and "The client in wave 1".
5. Point 4 of the epic's section "What the teams must agree on".
6. The findings of review round 1, listed in `attack/round-1-findings.md`.
7. The findings of review round 2, listed in `attack/round-2-findings.md`, and the orchestrator's ruling of 2026-11-26 in the task file of that fold.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the program and its settings | WS-1, item 1 | mapped |
| Brief item 2, the key of the machine | WS-2, item 3, on top of WS-1 item 2 | mapped |
| Brief item 3, the facts of the machine | WS-3, item 4 | mapped |
| Brief item 4, the harness scan | WS-3, item 5 | mapped |
| Brief item 5, events that wait for the server | WS-4, item 6 | mapped |
| Brief item 6, receipts of commands | WS-4, item 7, in the store of item 6 | mapped |
| Brief point 1, a full buffer | WS-4, item 7; decision 14 | mapped |
| Brief point 2, the size and the flushing of the buffer | WS-4, item 7; decision 15 | mapped |
| Brief point 3, the load on macOS without a shell | WS-3, item 4; decision 23 | mapped |
| Brief point 4, receipts in their own file | WS-4, item 7 in `receipts.go`; decisions 25 and 55 | mapped |
| Task point 1, the layout of the state folder | Implementation details, "The layout of the state folder"; WS-1 item 2; decision 9 | mapped |
| Task point 2, faking the installed programs | WS-3, item 5, "How the tests fake the installed programs"; decision 28 | mapped |
| Task point 3, surviving a crash in a write | WS-1 item 2 and WS-4 items 6 and 7, the crash tests; decisions 29 and 55 | mapped |
| Task point 4, depending on `go-s0-contract` only for `go.mod` | Rollout order step 1; decisions 18, 24 and 57. Round 2 adds the request for `go.etcd.io/bbolt` in `go.mod`. | mapped |
| D2, Go | every item | mapped |
| D7, a machine proves itself with a key pair | WS-2, item 3 | mapped |
| D8, ACP only | WS-3, item 5. The scan reports the plain programs, and wave 2 starts their ACP forms. | mapped |
| D16, no web server | WS-1, item 1. The program has no command that serves. | mapped |
| D19, one user on a machine | WS-1 item 2 and WS-2 item 3, as the accepted risk of an agent of the same user | mapped |
| R1, one folder for each part | every item keeps its own folder under `src/client/internal/` | mapped |
| R6, commands through the stored commands | WS-4, item 7 is the client's half | mapped |
| R9, text from outside is data, and secrets never reach a log | WS-2 item 3, WS-3 item 5, WS-4 item 7 | mapped |
| R10, real tests | Guardrails. No test here needs a database, and no test replaces a part of the client with a stand-in. | mapped |
| C7, facts, load and harnesses shown | WS-3, items 4 and 5, for the client's half | mapped |
| C10, an unknown key is refused | WS-2, item 3 makes the key that the server knows | mapped |
| C11, buffered events arrive once, in order | WS-4, item 6, for the client's half | mapped |
| Section "Joining", step 3 | WS-2, item 3 | mapped |
| Section "Commands go down" | WS-4, item 7 | mapped |
| Section "Events come up" | WS-4, item 6 | mapped |
| Section "What travels in wave 1" | WS-3, items 4 and 5 | mapped |
| Section "Knowing whether a machine is there" | WS-3, item 4, the load | mapped |
| Section "The client in wave 1" | WS-1, items 1 and 2; WS-3, item 5 | mapped |
| Epic agreement point 4, the layout of the state folder | Implementation details, "The layout of the state folder" | mapped |
| Round 1 findings fixed in this plan's items | WS-1 to WS-4, at the places the register's patch list names | mapped |
| Round 1 findings that the caller must keep | Fold "What plan go-s3-client-link must do" | mapped |
| Round 1 findings that need the built code | Fold "Post-build checks" | mapped |
| Round 1 findings for the owner | Section "For the owner"; decision 48, and decision 49 settled by decision 69 | mapped |
| Round 2 findings fixed in this plan's items | WS-1 to WS-4, at the places the register of round 2 names | mapped |
| Round 2 findings that the caller must keep | Fold "What plan go-s3-client-link must do" | mapped |
| Round 2 findings for the owner | Section "For the owner"; decision 56 | mapped |
| The orchestrator's ruling of round 2, one store with transactions | WS-4, items 6 and 7; the layout; decision 55 | mapped |

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
| 2. The state folder and safe writes | backend | client/backend | medium | A known method of writing in two small files, made risky by ordering and covered by named crash tests. Rounds 1 and 2 added small checks, each with its own test, and round 2 removed the two folders of the old outbox. | no | `go.mod` |
| 3. The key of the machine | backend | client/backend | medium | The standard library does the cryptography. The work is file handling with one test for each refusal. | no | 2 |
| 4. The facts and the load | backend | client/backend | medium | Two operating systems and a binary decoder, all pure functions with fixtures. | no | `go.mod` |
| 5. The harness scan | backend | client/backend | medium | Child processes under a time limit, a known pattern with clear tests. Rounds 1 and 2 added one small check of where a program and its helpers live. | no | `go.mod` |
| 6. The store and the outbox | backend | client/backend | medium | The store gives the atomic commit. The item adds one creation step, the numbers, the limits and four small transactions, each with its own test and a kill test. UNREVIEWED MECHANISM. | no | 2, and `go.etcd.io/bbolt` in `go.mod` |
| 7. Receipts of commands | backend | client/backend | medium | One record with six states in a bucket of item 6, where each rule is one transaction with its own test. UNREVIEWED MECHANISM. | no | 6 |

No unit is rated high. The brief rated items 2 and 5 high, and both were split by decision 7.
No unit changes an entry point that outsiders can reach, so no unit needs an attack pass.
The scope file `.plans/scopes/client/backend/scope.json` exists in the checkout, and every unit uses it (architect-r1 11).

Round 2 merged the three units of the old outbox and receipts into units 6 and 7, so the plan has seven units.

Load: each unit has one test writer, one builder and one code reviewer, and the reader of the third vendor beside the code review. The four risky units (2, 3, 6 and 7) add one break reviewer each. That is 7 test writers, 7 builders, 7 code reviewers, 7 readers and 4 break reviewers, 32 launches in all. With builders alternating between the two main vendors, one of them takes 13 launches and the other 12, and the third vendor takes the 7 reader seats. The orchestrator checks these counts against each vendor's weekly meter at the start of sprint 1, as the epic says. If one vendor would run out, the units of WS-3, which share nothing with the others, are the first to move later in the sprint.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 0 | none yet | The planner stopped after the self-check, as the shared rules for planners say. The orchestrator runs the review rounds with other vendors. |
| 1 | 57 findings from five reviewers: 18 blockers, 30 majors and 9 minors, in 26 groups, 11 of which hold more than one finding. | Every finding has an outcome in `attack/round-1-findings.md`. The promise that buffered events arrive once and in order (C11) is stuck, with 55 of 116 points. Three mechanisms are new and marked UNREVIEWED MECHANISM: the epoch of the outbox, the reconciliation with the server's number, and the rule for a trusted program. Two choices went to the owner. The next round attacks only the patch list of the register. |
| 2 | 32 findings from three reviewers: 9 blockers, 18 majors and 5 minors, in 19 groups. | Every finding has an outcome in `attack/round-2-findings.md`. C11 is stuck again, with 52 of 87 points. By the orchestrator's ruling, the outbox of files, epoch and mark was replaced by one store with transactions, and every finding in it is fixed another way under that ruling. One choice went to the owner. Round 3 attacks the store as a new design, and the patch list of the register. |

Known gaps from the self-check, for the reviewers:

- Gate 1: row 5 of `decisions.md` is open. It is the owner's decision on the cut of wave 1 into sprints, and the planner cannot close it. Decision 34 records that the plan builds on the cut as written.
- Gate 7: rounds 1 and 2 ran, and round 3 has not.
- Gate 10: the remaining weekly allowance of each vendor is not known to the planner.
- Items 6 and 7 need `go.etcd.io/bbolt` in `go.mod`, which the Contract team has not yet added.
- The comment of `Welcome.last_event_number` in the draft contract says that the client forgets kept events up to a higher number. `Reconcile` moves unsent events instead, and the result file asks the Contract team to change the comment.
- The owner check of item 3 has no test, because a test cannot make a file owned by another user without the rights of the superuser.
- macOS code is compiled and vetted on Linux, but its tests run only when a Mac is available in phase 5.

</details>
