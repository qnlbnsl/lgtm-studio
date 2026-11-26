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
A file that must never be replaced, such as an event or the key, is linked to its name instead of renamed over it.
A crash of the program then leaves either the old file or the new one, never a mix.
When the last sync fails, the write is reported as uncertain, and the record stops taking writes until it is opened again (critic-r1 1).
Tests prove the order by killing a child process at random moments and at each step of a write, and by checking the order of the syncs.
A real loss of power is checked after the build, in the section "Post-build checks".

The plan also fixes the layout of the state folder.
Plan `go-s3-client-link` reads the same files later, so every name, format and permission is written down here.

No new owner decision is needed to start.
The cut of wave 1 into sprints is still open with the owner, and this plan builds on it as written.
Review round 1 raised two choices for the owner, about a damaged outbox and about a wrong clock, and the section "For the owner" describes them.
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
A damaged receipt is kept for ever as "interrupted", so that no cleanup can make room for a second run (critic-r1 4).

**The outbox deletes only events that it numbered and the server applied.**
Round 1 found that one call both acknowledged events and recovered a lost outbox, and that a wrong number from the server could delete events never sent.
The plan now refuses an acknowledgement above the numbers the outbox handed out, and it checks the server's number in a separate step when the client connects.
Each outbox also gets a random name, its epoch, so that a lost outbox starts again at 1 under a new name instead of reusing numbers.
This needs a field in the contract, which the result file asks of the Contract team.

**The harness scan never runs a shell and never inherits secrets.**
It runs each program directly with the one argument `--version`, with only three variables in its environment, and with a time limit of 3 seconds.
It counts a program as installed only when its whole first line has the exact form of that harness's version, so nothing else a program prints can reach the website (threat-r1 10).
It never runs a program from a place that another user can change (threat-r1 06).
Tests use fake programs in a temporary folder, so they never touch the real programs on the machine.

**The load on macOS uses the load average.**
macOS gives no busy time of the processor to a Go program without a C library call.
The client therefore reports the load of the last minute divided by the number of cores, on Linux and on macOS alike.
A test compares the result with what macOS itself reports, on a real Mac in phase 5 of sprint 1, or of sprint 3 when no Mac is available in sprint 1 (critic-r1 9).

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

The client keeps the events that wait for the server in an outbox on disk, numbered from 1, until the server acknowledges them.
It deletes an event only when the server applied it, and it never accepts a number from the server above its own.
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
5. Run the whole suite of the client, the build for macOS and the post-build checks on the merged work, in phase 5 of the build loop.
6. Hand the layout of the state folder and the section "What plan go-s3-client-link must do" to plan `go-s3-client-link` before its planner freezes it.

Nothing in this plan is hard to reverse.
No step needs the owner.

## For the owner

Review round 1 left two choices that the design does not settle.
Neither blocks the build of sprint 1, because the plan takes the safe side of each until the owner rules.

**What happens when a file in the outbox is damaged.**
A damaged event file, for example after a disk fault, now stops the client, and nothing is deleted.
The owner can choose one of three ways forward.

1. Keep refusing, as the plan does now. Nothing is ever lost silently, but a machine with one bad file stays off the website until a person repairs it by hand. This costs nothing to build.
2. Move the damaged files to a folder `outbox/damaged/`, start again after them, and report the lost numbers to the server as a gap. The machine comes back by itself, and the website can show which events were lost. This costs a new event kind in the contract and about one day in plan `go-s3-client-link`.
3. Refuse until a person runs a repair command, which then does what option 2 does. A person sees every loss before it happens. This costs one more command in plan `go-s3-client-link`.

**What the client does when its clock is behind the server's.**
The server gives each command an expiry time, and the client compares it with its own clock.
A client clock that runs behind can run a command that the server already counts as expired.
The plan already stops a clock that is set back from undoing an expiry.
The owner can choose how to handle a clock that is simply wrong.

1. Trust the client's clock, and expect every machine to keep its time in sync. This costs nothing. A machine with a bad clock can run a stale command.
2. Let the server send how long a command may wait, and let the client count that time with its own steady clock. Clock errors then no longer matter. This costs a change to the contract and to the server.
3. Compare the two clocks when the client connects, and refuse commands while they differ by more than one minute. This costs a field in the link's greeting and a message on the website.

Round 1 also found that the promise that buffered events arrive once and in order drew almost half of all findings.
The register names that promise as stuck.
The plan now fixes each finding, but the next round should attack that part first.

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
- A test that needs a crash runs the test binary again as a child process. The child is started with `os.Executable()`, the argument `-test.run=^<helper>$`, and the variable `LGTM_TEST_HELPER` naming what it does. Each item has its own helper test, so two test writers never define the same name in one package: `TestHelperStateProcess` for item 2, `TestHelperOutboxProcess` for item 7 and `TestHelperReceiptsProcess` for item 8 (critic-r1 6). A helper returns at once when `LGTM_TEST_HELPER` does not name one of its modes, so it never runs on its own.
- The workstreams are numbered WS-1 to WS-4, in the order of the section "Workstreams" above.
- Every work item has the scope `client/backend` and the work area `backend`.

**The layout of the state folder**

Plan `go-s3-client-link` reads this layout. It is the agreement of point 4 in the epic's section "What the teams must agree on".

| Path inside the folder | Kind and mode | Format | Written by | Read by |
|---|---|---|---|---|
| (the folder) | folder, `0700` | | item 2, `state.Open` | every item |
| `lock` | file, `0600` | An empty regular file. A client holds an exclusive `flock` on it for as long as it uses the folder. `state.Open` refuses a `lock` that is a link, is not a regular file, has a bit for group or others, or belongs to another user (threat-r1 04). | item 2 | item 2 |
| `settings.json` | file, `0600` | JSON (JavaScript Object Notation): `{"version":1,"repositories_folder":"~/…","agent_limit":4}`. Both fields after `version` are optional. | the owner or the installer | item 1, `config.LoadSettings` |
| `machine.key` | file, `0600` | one PEM (Privacy-Enhanced Mail) block of type `PRIVATE KEY`, holding the Ed25519 key in PKCS #8 (Public-Key Cryptography Standards number 8) form, and at most one newline after it | item 3, `enroll.Create` | item 3, `enroll.Load` |
| `server.json` | file, `0600` | JSON with `"version":1`. Its other fields belong to plan `go-s3-client-link`: the server's address, its public key, the fingerprint of its certificate, and the machine's identifier. Plan `go-s3-client-link` writes it last in a join, and only after the server confirmed the join. A `machine.key` without a `server.json` therefore means a join that has not finished (architect-r1 7). | plan `go-s3-client-link`, through `state.WriteAtomic` | plan `go-s3-client-link` |
| `outbox/` | folder, `0700` | | item 2 | items 6 and 7 |
| `outbox/epoch` | file, `0600` | 32 lowercase hexadecimal characters and one newline. It names this run of event numbers. It is made once, when the outbox is new, and it is never replaced. UNREVIEWED MECHANISM, see item 6. | item 6 | items 6 and 7, and plan `go-s3-client-link`, which tells the server |
| `outbox/acked` | file, `0600` | One line, `v1 <number> <checksum>` and a newline. The number is the highest acknowledged event number in decimal. The checksum is the CRC-32C (cyclic redundancy check, Castagnoli polynomial) of the decimal text, in 8 lowercase hexadecimal characters (threat-r1 02). | item 6 | item 6 |
| `outbox/<number>.event` | file, `0600` | binary, described in item 6. The number has 20 digits with leading zeros, such as `00000000000000000042.event`. It is written with `state.CreateExclusive`, so it never replaces a file (critic-r1 1). | item 7 | items 6 and 7 |
| `receipts/` | folder, `0700` | | item 2 | item 8 |
| `receipts/<command id>.json` | file, `0600` | JSON, described in item 8 | item 8 | item 8 |
| `receipts/clock` | file, `0600` | The highest time the receipts have seen, as text in the form of RFC 3339 (Request for Comments 3339) with nanoseconds in UTC (Coordinated Universal Time), and one newline (logic2-r1 11). | item 8 | item 8 |
| `repositories/` | folder, `0700` | The default folder for clones. Item 1 only names it. Wave 2 creates and fills it (logic2-r1 12). | wave 2 | wave 2 |
| `.tmp-<random>` in any of these folders | file, `0600` | an unfinished write. Every opener removes it while it holds the lock. | item 2 | nobody |

The folder is found by this rule, from decision 8:

1. `LGTM_CLIENT_STATE_DIR`, when it is set and not empty. It must be an absolute path.
2. Otherwise `$XDG_STATE_HOME/lgtm-client`, when `XDG_STATE_HOME` is an absolute path. XDG is the X Desktop Group, whose rule for folders this follows.
3. Otherwise `$HOME/.local/state/lgtm-client`. This is the folder the design names.

Every folder above the state folder must be safe from other users, as item 2 checks (threat-r1 04).

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
- **Work area:** `backend`. **Complexity:** medium, because the method of writing is known and fits in two small files. Round 1 added the check of the parent folders, the syncs of new folders, the uncertain outcome and the claim of a store. Each is one check with its own test, so the item stays medium.
- **Scope:** `client/backend`.
- **Design statements:** D19 (the state belongs to one user), R9 (no other user reads the state), the section "Joining" step 3 (a file only its user can read), and the sections "Commands go down" and "Events come up", whose records stand on these writes.
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
- **Public names that others build against:**
  - Constants `LockFile = "lock"`, `SettingsFile = "settings.json"`, `KeyFile = "machine.key"`, `ServerFile = "server.json"`, `OutboxDir = "outbox"`, `ReceiptsDir = "receipts"`, `RepositoriesDir = "repositories"`, `TempPrefix = ".tmp-"`.
  - `type Paths struct { Dir, Lock, Settings, Key, Server, Outbox, Receipts, Repositories string }` and `func PathsIn(dir string) Paths`.
  - `type Folder struct { Paths Paths }` with `func Open(dir string) (*Folder, error)` and `func (f *Folder) Close() error`.
  - `func (f *Folder) Claim(store string) (release func(), err error)`. Items 7 and 8 call it with `"outbox"` and `"receipts"`. A second claim of the same name, before its release, returns `ErrStoreOpen`. `Close` returns `ErrStoreOpen` and keeps the lock while a claim is held. Writer: item 2. Readers: items 7 and 8 (logic-r1 3, architect-r1 5, critic-r1 5).
  - `func WriteAtomic(dir, name string, data []byte) error`, which writes mode `0600`.
  - `func CreateExclusive(dir, name string, data []byte) error`, which writes mode `0600` and returns an error that wraps `os.ErrExist` when the name exists.
  - `func RemoveTemps(dir string) error` and `func SyncDir(dir string) error`.
  - `var ErrNotAFolder`, `ErrFolderExposed`, `ErrFolderNotOwned`, `ErrFolderLocked`, `ErrBadName`, `ErrParentUnsafe`, `ErrLockUnsafe`, `ErrStoreOpen`, `ErrPlacedNotSynced`. Writer: item 2. Readers: items 3, 7 and 8, and plan `go-s3-client-link`.
  - Unexported, for tests in the package only: `var crashHook func(step string)`, `var syncFile func(*os.File) error`, `var syncFolder func(dir string) error`. The steps are `"temp-created"`, `"temp-written"`, `"temp-synced"`, `"placed"` and `"dir-synced"`, in that order. `"placed"` is the rename, or the link for `CreateExclusive`.
  - In `testhook.go`: `func SetCrashHookForTest(hook func(step string)) (restore func())` and `func SetSyncFolderForTest(sync func(dir string) error) (restore func())`. Each sets its variable and returns a function that puts the old one back. Their comments say that only tests call them. When nothing calls them, the writes call no hook and sync for real.
- **Behaviour:**
  - `Open` first resolves the part of the path that exists with `filepath.EvalSymlinks`. Every folder of that resolved path, from the root down, must belong to the user who runs the client or to the superuser. It must have no write bit for group or others, unless it belongs to the superuser and has the sticky bit, as `/tmp` does. Otherwise `Open` returns `ErrParentUnsafe` and creates nothing (threat-r1 04).
  - `Open` then creates each missing folder of the path, one at a time from the top, with mode `0700`. After it creates a folder, it syncs that folder's parent (architect-r1 1, critic-r1 2, logic-r1 4, threat-r1 03).
  - `Open` refuses the state folder when it is a symbolic link or not a folder (`ErrNotAFolder`), when its mode has any bit of group or others (`ErrFolderExposed`), and when its owner is not the user who runs the client (`ErrFolderNotOwned`). It never changes the mode of a folder that already exists.
  - `Open` opens `lock` with `O_RDWR|O_CREATE|O_NOFOLLOW` and mode `0600`. It checks the open file with `Stat`: a file that is not regular, has a bit of group or others, or belongs to another user gives `ErrLockUnsafe`. It then takes `syscall.Flock` with `LOCK_EX|LOCK_NB`. When another holder has it, `Open` returns `ErrFolderLocked`. The open file stays open until `Close`, so the lock stays on the same file (architect-r1 5).
  - Under the lock, `Open` creates `outbox/` and `receipts/` the same way, checks them as it checked the state folder, and calls `RemoveTemps` on all three folders.
  - Before it returns, `Open` always syncs the parent of the state folder, the state folder, `outbox/` and `receipts/`, even when it created nothing. A later `Open` therefore finishes an earlier one that stopped half-way. Any failed sync makes `Open` release the lock and return the error, and the caller gets no `Folder`.
  - `WriteAtomic` and `CreateExclusive` refuse with `ErrBadName` a name that is empty, `.` or `..`, holds a path separator, or starts with `TempPrefix`.
  - `WriteAtomic` creates `.tmp-<16 random hexadecimal characters>` with `O_CREATE|O_EXCL` and mode `0600`, writes the data, syncs the file, closes it, renames it over the name, and syncs the folder.
  - `CreateExclusive` does the same, except that it links the temporary file to the name with `os.Link`, which fails when the name exists, and then removes the temporary file.
  - Both have two kinds of failure (critic-r1 1, logic-r1 1, architect-r1 2, threat-r1 07, logic2-r1 8). A failure before the step `placed` changes nothing: the temporary file is removed and the final name is as before. A failure of the folder sync after `placed` returns an error that wraps `ErrPlacedNotSynced`. The new file is then in place in the running system, and it may or may not survive a loss of power. The caller must treat such a write as uncertain. Items 3, 7 and 8 say what each caller does with it. The removal of the temporary file is cleanup, and its failure never changes the result.
  - Each error message is one sentence that names the folder or the file.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `folder_test.go` | `TestOpenCreatesPrivateFolders` | `Open` on a path inside `t.TempDir()` that does not exist creates the folder, `outbox/` and `receipts/`, each with mode exactly `0700`, and `lock` with mode exactly `0600`. |
| `folder_test.go` | `TestPathsInNamesEveryFile` | `PathsIn("/s")` gives `/s/lock`, `/s/settings.json`, `/s/machine.key`, `/s/server.json`, `/s/outbox`, `/s/receipts` and `/s/repositories`. |
| `folder_test.go` | `TestOpenRefusesExposedFolder` | For an existing folder of mode `0750`, then `0705`, `Open` returns `ErrFolderExposed`, and the mode is unchanged afterwards. The same holds for `outbox/` of mode `0755` inside a folder of mode `0700`. |
| `folder_test.go` | `TestOpenRefusesSymlink` | When the path is a symbolic link to a folder of mode `0700`, `Open` returns `ErrNotAFolder`. |
| `folder_test.go` | `TestOpenRefusesWritableParent` | When a parent of the state folder has mode `0777` without the sticky bit, `Open` returns `ErrParentUnsafe`, and nothing is created inside it (threat-r1 04). |
| `folder_test.go` | `TestOpenRefusesUnsafeLock` | An existing `lock` that is a symbolic link, and one with mode `0644`, each give `ErrLockUnsafe` (threat-r1 04). |
| `folder_test.go` | `TestOpenSyncsNewFolderEntries` | With `syncFolder` replaced by a recorder, `Open` of `<tmp>/a/b/state`, where `a` and `b` do not exist, records a sync of `<tmp>`, `<tmp>/a` and `<tmp>/a/b`, and then of the state folder, `outbox/` and `receipts/`. With `syncFolder` failing for `<tmp>/a`, `Open` returns an error and holds no lock. A second `Open` with a working sync succeeds and records the syncs of the state folder's parent and of the three folders again (critic-r1 2). |
| `folder_test.go` | `TestOpenRefusesSecondHolder` | A second `Open` of the same folder, while the first is open, returns `ErrFolderLocked`. After the first `Close`, a new `Open` succeeds. The second `Open` also runs in a child process, so the test covers two processes and not only one. |
| `folder_test.go` | `TestLockIsFreedWhenHolderDies` | A child holds the folder open. After the parent kills it with `SIGKILL`, `Open` in the parent succeeds (critic-r1 5). |
| `folder_test.go` | `TestClaimAllowsOneStoreEach` | A second `Claim("outbox")` before the release returns `ErrStoreOpen`. After the release, a claim succeeds. `Close` while a claim is held returns `ErrStoreOpen`, and a second `Open` from a child still gets `ErrFolderLocked`. |
| `folder_test.go` | `TestOpenRemovesTemps` | A file `.tmp-abc` in the folder and one in `outbox/` are gone after `Open`. A file `keep.json` stays. |
| `atomic_test.go` | `TestWriteAtomicWritesPrivateFile` | After `WriteAtomic(dir, "a.json", data)`, the file holds exactly `data`, its mode is exactly `0600`, and no name with the prefix `.tmp-` is left. |
| `atomic_test.go` | `TestWriteAtomicReplaces` | Writing `A` and then `B` leaves exactly `B`. |
| `atomic_test.go` | `TestWriteAtomicRefusesBadNames` | `""`, `"."`, `".."`, `"a/b"` and `".tmp-x"` each return `ErrBadName`, and the folder is unchanged. |
| `atomic_test.go` | `TestWriteAtomicSyncsBeforeRename` | With `crashHook` set to record each step and `syncFile` and `syncFolder` replaced by recorders that note their call, the recorded order is `temp-created`, `temp-written`, the file sync, `temp-synced`, `placed`, the folder sync, `dir-synced`. The same order holds for `CreateExclusive`. |
| `atomic_test.go` | `TestWriteAtomicReportsPlacedNotSynced` | With `syncFolder` failing, `WriteAtomic` of `B` over `A` returns an error for which `errors.Is(err, ErrPlacedNotSynced)` is true, and the file holds `B`. With `syncFile` failing instead, the error is not `ErrPlacedNotSynced`, the file holds `A`, and no `.tmp-` name is left (critic-r1 1). |
| `atomic_test.go` | `TestCreateExclusiveReportsPlacedNotSynced` | With `syncFolder` failing, `CreateExclusive` of `k` returns `ErrPlacedNotSynced`, and `k` holds the data. |
| `atomic_test.go` | `TestCreateExclusiveNeverReplaces` | `CreateExclusive` of `k` with `A` succeeds. A second call with `B` returns an error for which `errors.Is(err, os.ErrExist)` is true, and the file still holds exactly `A`. No `.tmp-` name is left. |
| `crash_test.go` | `TestWriteAtomicCrashAtEachStep` | For each of the five steps, a child writes `x` with content `A`, then writes `x` with content `B` of 256 kibibytes, with `crashHook` set to `os.Exit(86)` at that step. The parent asserts that the child exited with 86. For `temp-created`, `temp-written` and `temp-synced`, `x` holds exactly `A`. For `placed` and `dir-synced`, `x` holds exactly `B` (critic-r1 6). After `RemoveTemps`, no `.tmp-` name is left. |
| `crash_test.go` | `TestCreateExclusiveCrashAtEachStep` | For each step, a child creates `k` with content `A` and exits with 86 at that step. For the first three steps, `k` is missing. For `placed` and `dir-synced`, it holds exactly `A`. |

The child processes of this item run the helper test `TestHelperStateProcess` in `crash_test.go`, which returns at once unless `LGTM_TEST_HELPER` names one of its modes (critic-r1 6).
A process kill leaves the operating system running, so these tests prove the order of the steps and not survival of a loss of power. The sync-order tests check the syncs that a loss of power needs, and a post-build check covers the rest.

- **Acceptance:** `go test -race ./src/client/internal/state/...` passes. The break reviewer's report names the nine mutations above and the test that went red for each.

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
| 2 | The program builds for Linux and macOS. | `go test -run TestBuildsForLinuxAndMacOS ./src/client/cmd/lgtm-client/` passes. It runs again on the merged work of all eight items in phase 5 (critic-r1 answer 5). |
| 3 | The state folder is created private, and a folder that others can read or change is refused. | `go test -run 'TestOpenCreatesPrivateFolders|TestOpenRefusesExposedFolder|TestOpenRefusesSymlink|TestOpenRefusesWritableParent|TestOpenRefusesUnsafeLock' ./src/client/internal/state/` passes. |
| 4 | Only one client uses a state folder, and only one store of each kind is open in it. | `go test -run 'TestOpenRefusesSecondHolder|TestLockIsFreedWhenHolderDies|TestClaimAllowsOneStoreEach' ./src/client/internal/state/` passes. |
| 5 | A crash in a write leaves the old file or the new one. | `go test -run 'TestWriteAtomicCrashAtEachStep|TestCreateExclusiveCrashAtEachStep' ./src/client/internal/state/` passes. |
| 6 | A new folder and a write whose folder sync failed are reported, not hidden. | `go test -run 'TestOpenSyncsNewFolderEntries|TestWriteAtomicReportsPlacedNotSynced|TestCreateExclusiveReportsPlacedNotSynced' ./src/client/internal/state/` passes. |

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
  - When `state.CreateExclusive` returns `ErrPlacedNotSynced`, `Create` returns that error and no credential. The key may be in place. The caller then calls `Load`, which reads that key, and never makes a second key (logic2-r1 8).
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

- **Files:** `src/client/internal/harness/registry.go`, `src/client/internal/harness/scan.go`, `src/client/internal/harness/trust.go`, `src/client/internal/harness/registry_test.go`, `src/client/internal/harness/scan_test.go`, `src/client/internal/harness/trust_test.go`.
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
  - In `trust.go`: `func trustedProgram(path string) (string, error)`, unexported, which returns the real path of a trusted program.
- **Behaviour:**
  - The programs are `claude` for Claude, `codex` for Codex, `gemini` for Gemini, and `cursor-agent` then `agent` for Cursor.
  - `Scan` first cleans `SearchPath`: it drops empty and relative entries and repeats. It uses the cleaned list both to find candidates and as the `PATH` of each child (architect-r1 9).
  - For each program name in order, and for each folder of the cleaned list in order, a candidate is the folder joined with the name. A candidate that does not exist is skipped silently.
  - A candidate is run only when `trustedProgram` accepts it (threat-r1 06). It resolves the candidate with `filepath.EvalSymlinks`. The real file must be regular and have an execute bit. The real file, the folder of the candidate in the search path, and every folder from the real file's folder up to the root must belong to the user who runs the client or to the superuser. None of them may have a write bit for group or others, except a folder of the superuser with the sticky bit. A candidate that fails this is not run, and it gets `ProblemUntrusted`. UNREVIEWED MECHANISM: the reviewer prescribed a trusted location without naming the rule. This rule is the one OpenSSH applies to its own files.
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
| `scan_test.go` | `TestScanNeverUsesShell` | The fake `claude` sits in a folder named `bin $(touch INJECTED);x`. It is found and installed, and no file `INJECTED` exists in `Home` or in the test's folders afterwards. |
| `scan_test.go` | `TestScanSkipsRelativeFolders` | With `SearchPath` holding `.` and the current folder of the test changed with `t.Chdir` to a folder with a fake `claude`, Claude is not installed. This test does not run in parallel. |
| `trust_test.go` | `TestScanSkipsNonExecutable` | A file `codex` of mode `0644` that prints a version gives not installed with `ProblemUntrusted`. |
| `trust_test.go` | `TestScanSkipsUntrustedFolder` | A valid fake in a folder of mode `0777` without the sticky bit is not run: its `.started` file does not exist, and the result is `ProblemUntrusted`. The same fake in a folder of mode `0755` is installed (threat-r1 06). |
| `trust_test.go` | `TestScanSkipsWritableProgram` | A valid fake of mode `0777` is not run and gives `ProblemUntrusted`. |
| `trust_test.go` | `TestScanFollowsLinkToCheck` | A link in a folder of mode `0755` that points at a fake in a folder of mode `0777` gives `ProblemUntrusted`. |
| `scan_test.go` | `TestScanTimesOut` | A fake `gemini` writes its `.started` file and then runs the absolute `sleep 30`. With `Timeout` 500 milliseconds, `Scan` returns within 3 seconds, Gemini is not installed with `ProblemTimedOut`, and the `.started` file exists (critic-r1 8). |
| `scan_test.go` | `TestScanDoesNotWaitForGrandchild` | A fake `gemini` starts the absolute `sleep 30` in the background with its output inherited, writes the process number of that `sleep` to a file, prints `0.68.0` and exits 0. `Scan` returns within 3 seconds with Gemini installed at `0.68.0`. At that moment the `sleep` process is still alive, which proves that it held the output. The test kills it in its cleanup. |
| `scan_test.go` | `TestScanBudgetCoversAllCandidates` | Fakes `cursor-agent` and `agent` in two folders both write `.started` and sleep. With `Timeout` 500 milliseconds, `Scan` returns within 3 seconds, and Cursor is not installed with `ProblemTimedOut` (architect-r1 9). |
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
| 5.3 | Write the check of where a program lives | `src/client/internal/harness/trust.go`, `trustedProgram` (target). Pattern: `Open` of task 2.1, which applies the same rule to folders (threat-r1 06). |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | The facts and the load are read on Linux. | `go test -run 'TestReadFactsOnLinux|TestReadLoadOnLinux' ./src/client/internal/sysinfo/` passes. |
| 2 | The values of macOS are decoded. | `go test -run 'TestDecodeDarwinLoadavg|TestDecodeDarwinUint64' ./src/client/internal/sysinfo/` passes, and `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet ./src/client/internal/sysinfo/` exits 0. |
| 3 | The scan finds fake harnesses and never touches the real ones. | `go test -run 'TestScanFindsFakePrograms|TestScanEmptySearchPathFindsNothing' ./src/client/internal/harness/` passes. |
| 4 | The scan runs no shell, runs no program from a place others can change, and leaks no secret. | `go test -run 'TestScanNeverUsesShell|TestScanRefusesExtraText|TestScanGivesMinimalEnvironment|TestScanChildPathIsCleaned|TestScanSkipsUntrustedFolder|TestScanSkipsWritableProgram|TestScanFollowsLinkToCheck' ./src/client/internal/harness/` passes. |
| 5 | A program that never answers does not stop the scan. | `go test -run 'TestScanTimesOut|TestScanDoesNotWaitForGrandchild|TestScanBudgetCoversAllCandidates' ./src/client/internal/harness/` passes. |
| 6 | On a real Mac, the load and the memory match what the system reports. | On a Mac, `go test -run TestReadLoadOnMacMatchesSystem ./src/client/internal/sysinfo/` passes (critic-r1 9). Phase 5 of sprint 1 runs it. When no Mac is available, phase 5 logs the gap, and phase 5 of sprint 3 runs it again. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A harness prints a secret with its version, and the secret reaches the website. | high | Only a whole first line that matches the harness's pattern counts, proven by `TestScanRefusesExtraText` (threat-r1 10). The child gets no variable of the client, proven by `TestScanGivesMinimalEnvironment`. |
| Another user plants a program with a harness's name in a folder they can write, and the scan runs it. | high | `trustedProgram` refuses a program or a folder that another user can change, proven by `TestScanSkipsUntrustedFolder` (threat-r1 06). |
| A program never ends, or a grandchild holds the output open. | medium | One time limit for all candidates of a harness, and `WaitDelay`, proven by `TestScanTimesOut`, `TestScanDoesNotWaitForGrandchild` and `TestScanBudgetCoversAllCandidates`. |
| Another program called `agent` is taken for Cursor. | medium | Cursor's pattern needs a version that starts with a date, proven by `TestCursorAgentNeedsCursorVersion`. |
| A harness changes the format of its version line, and shows as not installed. | medium | The result is visible as `ProblemNoVersion`, which is safe. A post-build check reads the real output of each harness. |
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

**Item 6. Event files, the epoch and the acknowledged mark**

- **Files:** `src/client/internal/link/eventfile.go`, `src/client/internal/link/eventfile_test.go`.
- **Needs first:** item 2. Runs at the same time as items 3 and 8.
- **Work area:** `backend`. **Complexity:** medium, because it is a small binary format, two small text files and one recovery function, with a fixture test for each state a crash or a damaged file can leave.
- **Scope:** `client/backend`.
- **Design statements:** the section "Events come up", and C11, whose "once, in order" stands on this recovery.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Skipping the check of the checksum of an event makes `TestDecodeEventRejectsFlippedByte` fail.
  2. Computing the next number from the event files alone, without the mark, makes `TestRecoverNextAfterAllAcked` fail.
  3. Accepting a gap in the numbers makes `TestRecoverRefusesGap` fail.
  4. Not removing temporary files makes `TestRecoverRemovesTemps` fail.
  5. Removing acknowledged files before every file is checked makes `TestRecoverChangesNothingWhenDamaged` fail (threat-r1 02).
  6. Dropping the checksum of the mark makes `TestAckedChecksumCatchesDigitChange` fail (threat-r1 02).
  7. Accepting the number 0 makes `TestDecodeEventRefusesZeroSeq` fail (logic2-r1 5).
- **Format of one event file:** 8 bytes of the text `LGTMEV01`; the event number as an unsigned 64-bit big-endian value; the length of the payload as an unsigned 32-bit big-endian value; the CRC-32C of the payload as an unsigned 32-bit big-endian value; then the payload. The header is 24 bytes. A number is at least 1 and below `maxSeq`, which is 2 to the power 62 (logic2-r1 5).
- **The epoch.** UNREVIEWED MECHANISM. Five reviewers found that one operation, `Ack`, both acknowledged sent events and recovered a lost outbox, and that this could delete events that were never sent. The reviewers prescribed a separate operation of reconciliation. This plan adds the epoch so that reconciliation never needs to move the counter. Every outbox gets a random name when it is made. The server keeps its last applied number for each epoch of a machine. A lost or new outbox has a new epoch, starts again at 1, and the server starts a new count for it. A server number above the client's numbers then means a fault, and the client stops instead of deleting anything. This needs a field in the link contract, which the result file asks of the Contract team.
- **Names that item 7 builds against (unexported, in package `link`):** `const eventSuffix = ".event"`, `const ackedFile = "acked"`, `const epochFile = "epoch"`, `const maxSeq = 1 << 62`, `func eventFileName(seq uint64) string`, `func encodeEvent(seq uint64, payload []byte) []byte`, `func decodeEvent(b []byte) (uint64, []byte, error)`, `type recovered struct { epoch string; acked uint64; pending []uint64; sizes map[uint64]int; next uint64 }`, `func recoverOutbox(dir string) (recovered, error)`, `func writeAcked(dir string, n uint64) error`, `func readAcked(dir string) (uint64, bool, error)`, where the `bool` says whether the file exists. Exported: `var ErrOutboxCorrupt`.
- **Behaviour:**
  - `eventFileName(42)` is `00000000000000000042.event`.
  - `decodeEvent` refuses a wrong magic text, a length that differs from the rest of the bytes, a checksum that does not match, the number 0 and a number of `maxSeq` or more, each with an error that wraps `ErrOutboxCorrupt`.
  - `writeAcked` writes `v1 <number> <checksum>` and a newline through `state.WriteAtomic`. `readAcked` gives an error that wraps `ErrOutboxCorrupt` for any other text, for a checksum that does not match, and for a number of `maxSeq` or more.
  - `recoverOutbox` first calls `state.RemoveTemps`. When the folder holds no epoch, no mark and no event file, the outbox is new. It then writes `epoch` with `state.CreateExclusive` from 16 random bytes in lowercase hexadecimal, writes the mark 0, and returns `next` 1.
  - Otherwise, before it changes anything, it checks everything (threat-r1 02). The epoch must exist and be 32 lowercase hexadecimal characters and a newline. The mark must exist and be valid. A missing epoch or a missing mark in an outbox that holds other files gives `ErrOutboxCorrupt`. Every event file above the mark must decode, and its header must agree with its name. The numbers above the mark must run without a gap from the mark plus 1 to the highest.
  - Only when all of that holds does it remove every event file whose number is at most the mark, and sync the folder. A file with another name is left alone. `next` is the larger of the mark and the highest number, plus 1.
  - On `ErrOutboxCorrupt`, nothing in the folder is removed or changed. What the client does then is a decision for the owner (logic2-r1 6). Until the owner rules, `OpenOutbox` fails, and plan `go-s3-client-link` stops `run` with one sentence that names the damaged file.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `eventfile_test.go` | `TestEventFileName` | 1 gives `00000000000000000001.event`, and `maxSeq` minus 1 gives 20 digits. |
| `eventfile_test.go` | `TestEncodeDecodeRoundTrip` | For payloads of 0 bytes, 1 byte and 1 mebibyte, decoding the encoding gives the same number and the same bytes. |
| `eventfile_test.go` | `TestDecodeEventRejectsFlippedByte` | Changing any one byte of the payload, the magic text or the length gives an error that wraps `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestDecodeEventRejectsTruncated` | Each length from 0 to the full length minus 1 gives an error. |
| `eventfile_test.go` | `TestDecodeEventRefusesZeroSeq` | An event encoded with the number 0, and one with `maxSeq`, each give `ErrOutboxCorrupt` (logic2-r1 5). |
| `eventfile_test.go` | `TestAckedRoundTrip` | `writeAcked(dir, 17)` then `readAcked(dir)` gives 17 and true. A missing file gives false. A file holding `x\n` gives `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestAckedChecksumCatchesDigitChange` | A mark written for 1 and then changed by hand to the number 9 with the checksum of 1 gives `ErrOutboxCorrupt` (threat-r1 02). |
| `eventfile_test.go` | `TestRecoverNewOutboxMakesEpochAndMark` | An empty folder gets an `epoch` file of 32 lowercase hexadecimal characters and a mark of 0, and gives no pending event and `next` 1. A second recovery of the same folder keeps the same epoch. |
| `eventfile_test.go` | `TestRecoverDeletesAcked` | Event files 1 to 5 and a mark of 3 give pending 4 and 5, `next` 6, and files 1 to 3 are gone. |
| `eventfile_test.go` | `TestRecoverNextAfterAllAcked` | An epoch, no event file and a mark of 9 give `next` 10. |
| `eventfile_test.go` | `TestRecoverRemovesTemps` | A file `.tmp-1234` holding half an event is gone, and it is not counted as pending. |
| `eventfile_test.go` | `TestRecoverRefusesGap` | An epoch, a mark of 0 and event files 1, 2 and 4 give `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestRecoverRefusesMissingEpoch` | A mark and event files without an epoch give `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestRecoverRefusesMissingMark` | An epoch and event files 2 to 4 without a mark give `ErrOutboxCorrupt` (threat-r1 02). |
| `eventfile_test.go` | `TestRecoverRefusesNameMismatch` | A file named `00000000000000000002.event` whose header says 3 gives `ErrOutboxCorrupt`. |
| `eventfile_test.go` | `TestRecoverChangesNothingWhenDamaged` | With a mark of 1, event files 1 to 4 and a flipped byte in event 3, recovery gives `ErrOutboxCorrupt`, and all four event files and the mark are still on disk with their bytes unchanged. |
| `eventfile_test.go` | `TestRecoverIgnoresOtherNames` | A file `notes.txt` stays and does not change the result. |

- **Acceptance:** `go test -race -run 'TestEventFile|TestEncode|TestDecode|TestAcked|TestRecover' ./src/client/internal/link/` passes. The break reviewer's report names the seven mutations and the test that went red for each.

**Item 7. The outbox queue**

- **Files:** `src/client/internal/link/outbox.go`, `src/client/internal/link/outbox_test.go`, `src/client/internal/link/outbox_crash_test.go`.
- **Needs first:** item 6.
- **Work area:** `backend`. **Complexity:** medium, because the format and the recovery are done in item 6, and this item adds the numbers, the limits, one lock in memory, one failed state and one check of the server's number. Each rule has its own test. The reviewer of the next round should say whether this is now high, and if so it splits into the queue and the reconciliation.
- **Scope:** `client/backend`.
- **Design statements:** the section "Events come up" (numbered, on disk, within a limit, sent again from the first one not acknowledged), and C11.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Returning the number from `Append` before the event file is durable makes `TestAppendReturnsOnlyAfterDurable` fail. `TestOutboxKillKeepsEveryNumberedEvent` is a second line of defence, not the proof (logic2-r1 7).
  2. Removing any acknowledged file before the mark is written makes `TestAckDeletesOnlyAfterMarkAtEveryRemoval` fail.
  3. Dropping the oldest event instead of refusing makes `TestAppendRefusesWhenFull` fail.
  4. Taking the next number from the count of pending events makes `TestOutboxNeverReusesNumbers` fail.
  5. Writing an event file with `state.WriteAtomic`, or going on after `ErrPlacedNotSynced`, makes `TestAppendAfterDirectorySyncErrorPreservesEvent` fail (critic-r1 1).
  6. Letting `Ack` accept a number that was never handed out makes `TestAckRefusesNumberNotHandedOut` fail (threat-r1 01).
  7. Letting `Reconcile` move the counter makes `TestReconcileCases` fail.
- **Public names that others build against:**
  - `type OutboxOptions struct { MaxEvents int; MaxBytes int64; MaxEventBytes int }`, with `DefaultMaxEvents = 10000`, `DefaultMaxBytes = 64 << 20` and `DefaultMaxEventBytes = 1 << 20`. A field of 0 means its default, and a negative field is refused with an error.
  - `type Event struct { Seq uint64; Payload []byte }`.
  - `type Outbox struct` and `func OpenOutbox(f *state.Folder, opts OutboxOptions) (*Outbox, error)`. It claims `"outbox"` on the folder and works in `f.Paths.Outbox` (logic-r1 3, architect-r1 5, critic-r1 5, logic2-r1 10).
  - `func (o *Outbox) Append(payload []byte) (uint64, error)`.
  - `func (o *Outbox) Read(from uint64, limit int) ([]Event, error)`.
  - `func (o *Outbox) Ack(upTo uint64) error`.
  - `func (o *Outbox) Reconcile(serverLast uint64) error`. UNREVIEWED MECHANISM, with the epoch of item 6.
  - `func (o *Outbox) Epoch() string`, `Acked() uint64`, `Next() uint64`, `Len() int`, `Bytes() int64`.
  - `func (o *Outbox) Changed() <-chan struct{}`.
  - `func (o *Outbox) Close() error`.
  - `var ErrOutboxFull`, `ErrEventTooLarge`, `ErrOutboxClosed`, `ErrOutboxFailed`, `ErrAckAhead`, `ErrServerBehind`, `ErrServerAhead`, `ErrSequenceExhausted`, and `ErrOutboxCorrupt` from item 6. Writer: item 7. Reader: plan `go-s3-client-link`.
- **Behaviour:**
  - `OpenOutbox` runs `recoverOutbox` and keeps the epoch, the mark, the pending numbers, their sizes and `next` in memory.
  - `Append` refuses a payload larger than `MaxEventBytes` with `ErrEventTooLarge`. It refuses with `ErrOutboxFull` when the pending count would pass `MaxEvents` or the pending bytes would pass `MaxBytes`. It refuses with `ErrSequenceExhausted` when `next` has reached `maxSeq`. A refusal changes nothing on disk or in memory.
  - `Append` writes the event file for `next` with `state.CreateExclusive`, so that it can never replace an event (critic-r1 1). Only when that write succeeds does it take the number, add 1 to `next`, and return the number. It then signals `Changed()` without blocking.
  - When a write of an event or of the mark fails with `ErrPlacedNotSynced`, or when the name of the next event already exists, the outbox enters a failed state (architect-r1 2, critic-r1 1, logic-r1 1, threat-r1 07). The call returns an error that wraps both `ErrOutboxFailed` and the cause. Every later `Append`, `Read`, `Ack` and `Reconcile` returns an error that wraps `ErrOutboxFailed`. Recovery is to close the outbox and open it again. The reopen reads the disk, so an event that was placed is then pending under its number, or absent after a loss of power, and no number is given to two events. The producer of the failed `Append` must treat its event as possibly stored.
  - `Read(from, limit)` returns, in order, at most `limit` pending events whose number is at least the larger of `from` and the mark plus 1. A `limit` of 0 or less returns nothing. `Read`, `Ack` and `Reconcile` hold the same lock, and `Read` copies every payload while it holds it (logic2-r1 10).
  - `Ack(upTo)` acknowledges only numbers this outbox handed out (architect-r1 3, critic-r1 3, logic-r1 5, logic2-r1 1, threat-r1 01). It does nothing when `upTo` is at most the mark. It returns `ErrAckAhead` and changes nothing when `upTo` is `Next()` or more. Otherwise it writes the mark with `writeAcked` first, then removes the event files up to `upTo` one by one, and then syncs the folder. A failure to remove a file after the mark is written is not returned, because the next `Ack` or `OpenOutbox` removes it.
  - `Reconcile(serverLast)` takes the number the server says it applied for this epoch. It is called once on each connection, after both ends have proven themselves. When `serverLast` is below the mark, it returns `ErrServerBehind` and changes nothing. When `serverLast` is at least the mark and below `Next()`, it acts as `Ack(serverLast)`. When `serverLast` is `Next()` or more, it returns `ErrServerAhead` and changes nothing. The outbox never moves its counter from a number that the server sends.
  - All methods are safe to call at the same time.
  - `Close` closes the channel of `Changed()`, so that every receiver wakes, and then releases the claim. A second `Close` returns nil. After `Close`, `Append`, `Read`, `Ack` and `Reconcile` return `ErrOutboxClosed`, and `Epoch`, `Acked`, `Next`, `Len` and `Bytes` return their last values (critic-r1 10).
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `outbox_test.go` | `TestAppendNumbersFromOne` | Three appends return 1, 2 and 3. `Read(0, 10)` returns the three payloads in order. |
| `outbox_test.go` | `TestReopenKeepsPending` | After three appends, `Close` and `OpenOutbox` again, `Read(0, 10)` returns the same three events, `Epoch()` is the same, and the next append returns 4. |
| `outbox_test.go` | `TestReadFromFirstUnacked` | After five appends and `Ack(2)`, `Read(0, 10)` returns 3, 4 and 5. `Read(4, 10)` returns 4 and 5. `Read(0, 2)` returns 3 and 4. |
| `outbox_test.go` | `TestAckIsCumulativeAndIdempotent` | After five appends and `Ack(3)`, `Ack(2)` and `Ack(3)` return nil and change nothing. `Acked()` is 3. |
| `outbox_test.go` | `TestAckRefusesNumberNotHandedOut` | With events 1 and 2 pending, `Ack(3)` and `Ack(math.MaxUint64)` each return `ErrAckAhead`, both events stay pending, and the next append returns 3 (threat-r1 01). |
| `outbox_test.go` | `TestReconcileCases` | With the mark 2 and events 3 to 5 pending, `Reconcile(1)` gives `ErrServerBehind`, `Reconcile(2)` gives nil and changes nothing, `Reconcile(4)` leaves only event 5 pending, and `Reconcile(6)` and `Reconcile(math.MaxUint64)` give `ErrServerAhead`. After each refusal, the pending events and `Next()` are unchanged. |
| `outbox_test.go` | `TestNewOutboxKeepsEventsBeforeFirstReconcile` | On a new outbox, two appends return 1 and 2. `Reconcile(0)` gives nil, and `Read(0, 10)` returns both. `Reconcile(500)` gives `ErrServerAhead`, and both events are still pending (critic-r1 3, logic2-r1 1). |
| `outbox_test.go` | `TestOutboxNeverReusesNumbers` | After five appends, `Ack(5)`, a reopen, and one append, the number is 6. |
| `outbox_test.go` | `TestAppendReturnsOnlyAfterDurable` | With `state.SetCrashHookForTest` blocking at the step `dir-synced`, an `Append` started in a goroutine has not returned after 200 milliseconds. After the hook is released, it returns the next number (logic2-r1 7). |
| `outbox_test.go` | `TestAppendAfterDirectorySyncErrorPreservesEvent` | After five appends, `state.SetSyncFolderForTest` makes the folder sync fail. `Append(A)` returns an error that wraps `ErrPlacedNotSynced` and `ErrOutboxFailed`. `Append(B)` returns `ErrOutboxFailed`. After the sync works again, `Close` and `OpenOutbox`, event 6 is pending with payload `A`, and the next append returns 7 (critic-r1 1). |
| `outbox_test.go` | `TestAckAfterDirectorySyncError` | With five events and a failing folder sync during the write of the mark, `Ack(3)` returns `ErrPlacedNotSynced`, and later calls return `ErrOutboxFailed`. After a reopen with a working sync, the mark is 3, events 4 and 5 are pending, and no event at or below the mark is pending. |
| `outbox_test.go` | `TestAckDeletesOnlyAfterMarkAtEveryRemoval` | After five appends, the test replaces `removeFile` with a recorder that calls `readAcked` at every call. During `Ack(3)`, every call sees the mark 3 already on disk, and the files of events 1 to 3 are gone afterwards (logic2-r1 7). |
| `outbox_test.go` | `TestAppendRefusesWhenFull` | With `MaxEvents` 3, the fourth append returns `ErrOutboxFull`, `Len()` stays 3, and events 1 to 3 are unchanged. After `Ack(1)`, an append succeeds and returns 4. With `MaxBytes` 10, appends of 6 bytes and then 5 bytes give success and then `ErrOutboxFull`. |
| `outbox_test.go` | `TestAppendRefusesLargeEvent` | With `MaxEventBytes` 8, a payload of 9 bytes gives `ErrEventTooLarge`, and no file is written. |
| `outbox_test.go` | `TestAppendRefusesAtSequenceCeiling` | A fixture outbox with the mark `maxSeq` minus 2 and event `maxSeq` minus 1 gives `ErrSequenceExhausted` on the next append, and nothing changes (logic2-r1 5). |
| `outbox_test.go` | `TestChangedSignalsAppend` | After an append, a receive from `Changed()` succeeds within 1 second. |
| `outbox_test.go` | `TestConcurrentAppendsAreUnique` | 8 goroutines append 50 events each. The 400 numbers are 1 to 400 with no repeat, and `Read(0, 1000)` returns 400 events in order. |
| `outbox_test.go` | `TestReadDuringAckNeverFails` | For 1,000 rounds, one goroutine appends and acknowledges while another reads. No `Read` returns an error, and no event it returns is at or below the mark it saw (logic2-r1 10). |
| `outbox_test.go` | `TestSecondOpenOutboxRefused` | A second `OpenOutbox` on the same `state.Folder` returns `ErrStoreOpen`. After `Close`, it succeeds. |
| `outbox_test.go` | `TestCloseWakesChangedAndKeepsSnapshot` | A goroutine waiting on `Changed()` returns within 1 second of `Close`. `Epoch`, `Acked`, `Next`, `Len` and `Bytes` return their values from before `Close`. `Append`, `Read`, `Ack` and `Reconcile` return `ErrOutboxClosed`. A second `Close` returns nil (critic-r1 10). |
| `outbox_crash_test.go` | `TestOutboxKillKeepsEveryNumberedEvent` | A child appends events in a loop. The payload of event `n` is the text `event-<n>` repeated up to a size between 1 byte and 64 kibibytes that depends on `n`. After each append returns, it prints `appended <n>`, and it acknowledges every tenth number. The parent waits for the child's first `appended` line, then waits a random 0 to 30 milliseconds, then kills it with `SIGKILL`. It does this 30 times in a row, from a fixed seed that the test logs, and the 30 runs must print at least 100 numbers in all (critic-r1 6). After each kill, `OpenOutbox` succeeds. Every number that the child printed is either at most the mark or pending with its exact payload. The pending numbers run without a gap from the mark plus 1. The first number the next child prints is larger than every number printed before. |
| `outbox_crash_test.go` | `TestOutboxCrashAtEachWriteStep` | For each of the five steps of a safe write, a child appends events 1 to 5, sets the hook, and then exits with 86 at that step during event 6. After `OpenOutbox`, events 1 to 5 are pending with their exact payloads. For the steps before `placed`, event 6 is missing and the next append returns 6. For `placed` and `dir-synced`, event 6 is pending with its exact payload and the next append returns 7 (critic-r1 6). This test is also the proof of the accepted repeat in logic2-r1 9: an event that was placed but whose `Append` never returned is sent. |
| `outbox_crash_test.go` | `TestOutboxCrashDuringAck` | For each of the five steps of the write of the mark, and for a stop after the mark and before each of the three removals, a child with five events exits with 86 during `Ack(3)`. After `OpenOutbox`, either the mark is 3 and events 4 and 5 are pending, or the mark is 0 and events 1 to 5 are pending. The next append returns 6 (logic2-r1 7). |

The child processes of items 7 and 8 run separate helper tests, `TestHelperOutboxProcess` in `outbox_crash_test.go` and `TestHelperReceiptsProcess` in `receipts_crash_test.go`, so that two test writers never define the same name in package `link` (critic-r1 6).
Each child sets the hook with `state.SetCrashHookForTest` only after its setup, so the hook fires during the write under test alone.
`removeFile` is an unexported variable of package `link`, `var removeFile = os.Remove`, which only tests replace.

- **Acceptance:** `go test -race -run 'TestAppend|TestReopen|TestRead|TestAck|TestReconcile|TestNewOutbox|TestOutbox|TestChanged|TestConcurrent|TestSecondOpenOutbox|TestClose' ./src/client/internal/link/` passes. The break reviewer's report names the seven mutations and the test that went red for each.

**Item 8. Receipts of commands**

- **Files:** `src/client/internal/link/receipts.go`, `src/client/internal/link/receipts_test.go`, `src/client/internal/link/receipts_crash_test.go`.
- **Needs first:** item 2. Runs at the same time as items 3 and 6.
- **Work area:** `backend`. **Complexity:** medium, because it is one record with five states and a crash test, built on the safe write of item 2. Round 1 added the check of each file, a clock that never goes back, and a failed state. The reviewer of the next round should say whether this is now high.
- **Scope:** `client/backend`.
- **Design statements:** the section "Commands go down" (a command that arrives twice runs once and the result is sent again, and a command past its expiry is not run), R6 and R9.
- **Risky:** yes, for ordering. These mutations must each make a named test fail:
  1. Writing the `started` receipt after `Begin` returns instead of before makes `TestBeginWritesStartedBeforeReturn` fail, and the count of effects in `TestReceiptsKillNeverRunsTwice` (logic-r1 answer 6).
  2. Leaving `started` receipts alone at open makes `TestOpenMarksStartedAsInterrupted` fail.
  3. Skipping the check of the expiry makes `TestBeginRefusesExpired` fail.
  4. Skipping the check of the identifier makes `TestBeginRejectsBadIDs` fail.
  5. Pruning a damaged receipt makes `TestDamagedReceiptIsNeverPruned` fail (critic-r1 4).
  6. Using the plain clock without the high-water mark makes `TestClockHighWaterSurvivesRollback` fail (logic2-r1 11).
  7. Running a higher attempt of a command that ended makes `TestReplayTruthTable` fail (logic2-r1 3).
- **Public names that others build against:**
  - `type Command struct { ID string; Attempt uint32; ExpiresAt time.Time }`.
  - `type ReceiptState string` with `ReceiptStarted = "started"`, `ReceiptDone = "done"`, `ReceiptFailed = "failed"`, `ReceiptInterrupted = "interrupted"`, `ReceiptExpired = "expired"`. The last four are the outcome that plan `go-s3-client-link` sends on a replay (logic2-r1 3). The state `finished` of round 0 is replaced by `done` and `failed`.
  - `type Receipt struct { CommandID string; Attempt uint32; ExpiresAt time.Time; State ReceiptState; StartedAt time.Time; FinishedAt time.Time; Result []byte; Note string; Pinned bool }`. `Note` is empty, `result too large` or `damaged receipt`. `Pinned` is true for a receipt that `Prune` never removes.
  - `type Decision int` with `DecisionRun Decision = 1`, `DecisionReplay Decision = 2`, `DecisionRunning Decision = 3`.
  - `type ReceiptsOptions struct { Now func() time.Time; Keep time.Duration; MaxResultBytes int }`, with `DefaultReceiptKeep = 24 * time.Hour` and `DefaultMaxResultBytes = 1 << 20`. A field left empty means its default, and `Now` defaults to `time.Now`.
  - `type Receipts struct` and `func OpenReceipts(f *state.Folder, opts ReceiptsOptions) (*Receipts, error)`. It claims `"receipts"` on the folder and works in `f.Paths.Receipts` (logic-r1 3).
  - `func (r *Receipts) Begin(c Command) (Decision, Receipt, error)`.
  - `func (r *Receipts) Finish(id string, state ReceiptState, result []byte) (Receipt, error)`, where `state` is `ReceiptDone` or `ReceiptFailed`.
  - `func (r *Receipts) Get(id string) (Receipt, bool, error)`.
  - `func (r *Receipts) Prune() (int, error)`.
  - `func (r *Receipts) Close() error`.
  - `var ErrInvalidCommand`, `ErrNotStarted`, `ErrResultTooLarge`, `ErrReceiptsClosed`, `ErrReceiptsFailed`. Writer: item 8. Reader: plan `go-s3-client-link`.
- **Format of one receipt file:** `receipts/<command id>.json`, mode `0600`, written with `state.WriteAtomic`. It is a JSON object with `version` (always 1), `command_id`, `attempt`, `expires_at`, `state`, `started_at`, `finished_at`, `result`, `note` and `pinned`. The times are RFC 3339 text with nanoseconds, in UTC. `finished_at` is left out while it is zero. `result` is the base64 of the bytes, and it is left out when empty. `note` and `pinned` are left out when empty or false.
- **The clock of the receipts:** every decision uses `now`, the later of `Now()` and the time in `receipts/clock` (logic2-r1 11). `OpenReceipts` reads that file, and the receipts write it through `state.WriteAtomic` whenever `now` has moved more than one minute past the stored time. A clock that is set back therefore never makes an expired command run, and never shortens the life of a receipt. A client clock behind the server's is a separate question for the owner, recorded in `decisions.md`.
- **Behaviour:**
  - `Begin` refuses with `ErrInvalidCommand` an `ID` that is not 36 characters of lowercase hexadecimal digits and hyphens in the canonical form of a universally unique identifier (UUID), an `Attempt` of 0, and a zero `ExpiresAt`. It writes nothing then.
  - When a receipt exists in the state `started` in this process, `Begin` returns `DecisionRunning` and that receipt. The caller sends nothing now, because `Finish` sends the result later.
  - When a receipt exists in any other state, `Begin` returns `DecisionReplay` and the receipt, whatever the attempt and the expiry of the arriving command. A command that ended never runs again under the same identifier, and the server sends a new identifier to try again (logic2-r1 3). When the arriving `ExpiresAt` is later than the stored one, `Begin` first raises the stored expiry on disk, so that the receipt lives as long as any copy of the command can arrive (architect-r1 4).
  - When no receipt exists and `now` is at or after `ExpiresAt`, `Begin` writes a receipt in the state `expired` and returns `DecisionReplay` with it (threat-r1 08).
  - Otherwise `Begin` writes a receipt in the state `started`, and only then returns `DecisionRun`.
  - `Finish` refuses with `ErrNotStarted` an identifier whose receipt is not `started` in this process. A result above `MaxResultBytes` is recorded as `failed` with an empty result and the note `result too large`, and `Finish` returns that receipt with `ErrResultTooLarge`, so the receipt never stays `started` (logic-r1 8). Otherwise it writes the receipt in the given state with the result and the time, and only then returns it.
  - When a write of `Begin` or `Finish` fails with `ErrPlacedNotSynced`, the receipts enter a failed state, as the outbox does. The call returns an error that wraps `ErrReceiptsFailed` and the cause, and the command is not run. Every later `Begin` and `Finish` returns `ErrReceiptsFailed` until the receipts are closed and opened again (architect-r1 2, threat-r1 07).
  - `OpenReceipts` removes temporary files and reads every receipt file whose name is a valid identifier. A file that cannot be read for a reason other than its absence makes `OpenReceipts` fail, so no command runs on a store it cannot see (logic-r1 2).
  - A receipt is valid only when its JSON decodes with no unknown field, `version` is 1, `command_id` equals the file name, `state` is one of the five, `attempt` is at least 1, `expires_at` and `started_at` are not zero, and `finished_at` is not zero for the four ended states (architect-r1 4, critic-r1 4, logic-r1 2, threat-r1 05).
  - `OpenReceipts` rewrites a receipt that is not valid as `interrupted`, with the note `damaged receipt`, `pinned` true, and the identifier from its name. It keeps every time it could decode. It rewrites every valid receipt in the state `started` as `interrupted`, with an empty result. It then runs `Prune`.
  - `Prune` removes every receipt that is not `started` and not pinned, and whose latest time of `ExpiresAt`, `StartedAt` and `FinishedAt` is more than `Keep` before `now`. It returns the number removed. A pinned receipt is removed only by a person.
  - A replayed result is the result of the attempt that ran it. Plan `go-s3-client-link` labels the reply with the attempt of the command that just arrived, because the server ignores a result for an older attempt.
  - All methods are safe to call at the same time. After `Close`, every method returns `ErrReceiptsClosed`, and `Close` releases the claim. A second `Close` returns nil.
- **Tests written first:**

| Test file | Test | What it asserts |
|---|---|---|
| `receipts_test.go` | `TestBeginRunsNewCommand` | A new valid command gives `DecisionRun`, and the file `<id>.json` exists with the state `started`. |
| `receipts_test.go` | `TestBeginWritesStartedBeforeReturn` | With `state.SetCrashHookForTest` blocking at the step `dir-synced`, a `Begin` started in a goroutine has not returned after 200 milliseconds. After the release, it returns `DecisionRun`. |
| `receipts_test.go` | `TestSecondArrivalReplaysResult` | After `Begin` and `Finish(id, ReceiptDone, "ok")`, a second `Begin` with attempt 2 gives `DecisionReplay`, the state `done`, the result `ok` and the attempt 1. |
| `receipts_test.go` | `TestArrivalWhileRunning` | A second `Begin` before `Finish` gives `DecisionRunning`. |
| `receipts_test.go` | `TestReplayTruthTable` | For the stored states none, `started` in this process, `done`, `failed`, `interrupted` and `expired`, crossed with the same and a higher attempt, and with the expiry before and after `now`: none with a later expiry gives `DecisionRun`; none with a past expiry gives `DecisionReplay` with `expired`; `started` gives `DecisionRunning`; every ended state gives `DecisionReplay` with that state. No ended state ever gives `DecisionRun` (logic2-r1 3). |
| `receipts_test.go` | `TestBeginRefusesExpired` | With `Now` fixed at noon and `ExpiresAt` one second before noon, `Begin` gives `DecisionReplay` with the state `expired`, and a second `Begin` gives the same. |
| `receipts_test.go` | `TestBeginExpiresAtTheInstant` | With `Now` equal to `ExpiresAt`, `Begin` gives `DecisionReplay` with `expired` (threat-r1 08). |
| `receipts_test.go` | `TestBeginRaisesStoredExpiry` | After a finished command with an expiry at noon, a second `Begin` with an expiry at 14:00 gives `DecisionReplay`, and the file on disk then holds the expiry 14:00 (architect-r1 4). |
| `receipts_test.go` | `TestFinishedSurvivesReopen` | After `Finish`, `Close` and `OpenReceipts`, `Begin` gives `DecisionReplay` with the same result. |
| `receipts_test.go` | `TestOpenMarksStartedAsInterrupted` | After `Begin` with no `Finish`, `Close` and `OpenReceipts`, `Get` gives the state `interrupted`, and `Begin` gives `DecisionReplay`, not `DecisionRun`. `Finish` gives `ErrNotStarted`. |
| `receipts_test.go` | `TestDamagedReceiptIsNeverPruned` | A file `<valid id>.json` holding `{` becomes `interrupted`, pinned, with the note `damaged receipt`. With `Now` ten years later, `Prune` keeps it. After a second and a third `OpenReceipts`, `Begin` of that identifier with an expiry in the future gives `DecisionReplay` with `interrupted` each time (critic-r1 4). |
| `receipts_test.go` | `TestValidJSONWithBadContentIsDamaged` | Files with no `state`, with `version` 2, with a `command_id` that differs from the name, with the state `weird`, and with a zero `started_at` each become `interrupted` and pinned, and `Begin` never gives `DecisionRun` for them (logic-r1 2). |
| `receipts_test.go` | `TestOpenFailsOnUnreadableReceipt` | A receipt file of mode `0000` makes `OpenReceipts` return an error. The test is skipped when it runs as the superuser. |
| `receipts_test.go` | `TestClockHighWaterSurvivesRollback` | With `Now` at T plus 48 hours, a command with an expiry at T plus 49 hours runs and finishes. With `Now` at T plus 74 hours, `Prune` removes it. With `Now` set back to T, a `Begin` of the same identifier with the expiry T plus 49 hours gives `DecisionReplay` with `expired`, not `DecisionRun` (logic2-r1 11). |
| `receipts_test.go` | `TestBeginRejectsBadIDs` | `../machine.key`, `a/b`, an uppercase UUID, a UUID without hyphens, and the empty text each give `ErrInvalidCommand`, and no file is written in the folder or beside it. An attempt of 0 and a zero expiry give the same. |
| `receipts_test.go` | `TestFinishTooLargeIsTerminal` | With `MaxResultBytes` 4, `Finish` with 5 bytes returns `ErrResultTooLarge` and a receipt in the state `failed` with the note `result too large`. A later `Begin` gives `DecisionReplay` with `failed` (logic-r1 8). |
| `receipts_test.go` | `TestBeginAfterDirectorySyncError` | With a failing folder sync, `Begin` of a new command returns an error that wraps `ErrPlacedNotSynced` and `ErrReceiptsFailed`, and not `DecisionRun`. The next `Begin` returns `ErrReceiptsFailed`. After a reopen with a working sync, `Begin` of that identifier gives `DecisionReplay` with `interrupted`. |
| `receipts_test.go` | `TestPruneRemovesOldReceipts` | With `Keep` one hour, a done receipt whose times are all two hours before `now` is removed, and one whose expiry is in 30 minutes stays. A `started` receipt and a pinned receipt are never removed. `Prune` returns 1. |
| `receipts_test.go` | `TestReceiptFileFormat` | The written file decodes as JSON with exactly the fields of the format above, `version` is 1, the times end in `Z`, and the mode is `0600`. |
| `receipts_test.go` | `TestConcurrentBeginRunsOnce` | 16 goroutines call `Begin` with the same command. Exactly one gets `DecisionRun`. |
| `receipts_test.go` | `TestSecondOpenReceiptsRefused` | A second `OpenReceipts` on the same `state.Folder` returns `ErrStoreOpen`. |
| `receipts_crash_test.go` | `TestReceiptsKillNeverRunsTwice` | A child loops over identifiers made from a counter. For each, it calls `Begin`. After `DecisionRun` it appends the identifier as one line to the file `effects` with a sync, prints `begun <id>`, calls `Finish` with `ReceiptDone` and the result `result-<id>`, and prints `finished <id>` after it returns. The parent waits for the first `begun` line, then a random 0 to 30 milliseconds, then kills the child with `SIGKILL`, 30 times in a row, from a fixed seed that the test logs. After each kill, no identifier appears twice in `effects` across all the runs. Every identifier printed as `finished` has the state `done` and the exact result. Every identifier in `effects` has a receipt in the state `done` or `interrupted`, never none. `Begin` of each identifier in `effects` never gives `DecisionRun`. |
| `receipts_crash_test.go` | `TestReceiptsCrashAtEachWriteStep` | For each step of a safe write, a child exits with 86 at that step while it writes the `started` receipt. After `OpenReceipts`, for the steps before `placed`, `Begin` of that identifier gives `DecisionRun`. For `placed` and `dir-synced`, it gives `DecisionReplay` with `interrupted`. The same is done for the step of `Finish`. The receipt is then `interrupted` for the steps before `placed`, and `done` with the exact result for `placed` and `dir-synced`. |

- **Acceptance:** `go test -race -run 'TestBegin|TestSecond|TestArrival|TestReplay|TestFinish|TestOpen|TestDamaged|TestValidJSON|TestClock|TestPrune|TestReceipt|TestConcurrentBegin' ./src/client/internal/link/` passes. The break reviewer's report names the seven mutations and the test that went red for each.

**Tasks of WS-4:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 6.1 | Write the format of an event file, the epoch and the mark | `src/client/internal/link/eventfile.go`, `encodeEvent`, `decodeEvent`, `writeAcked` and `readAcked` (target). Pattern: the design's section "Events come up". |
| 6.2 | Write the recovery of the outbox folder | `src/client/internal/link/eventfile.go`, `recoverOutbox` (target). Pattern: `Open` of task 2.1, which also removes temporary files under the lock (critic-r1 12). |
| 7.1 | Write the queue | `src/client/internal/link/outbox.go`, `OpenOutbox`, `Append`, `Ack` and `Reconcile` (target). Pattern: `floorplan/plan.json`, entry "Events waiting to be sent". |
| 8.1 | Write the receipts | `src/client/internal/link/receipts.go`, `OpenReceipts`, `Begin` and `Finish` (target). Pattern: the design's section "Commands go down". |

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | Every event that got a number survives a kill, whole and in order. | `go test -run 'TestOutboxKillKeepsEveryNumberedEvent|TestAppendReturnsOnlyAfterDurable' ./src/client/internal/link/` passes. |
| 2 | A crash at any step of a write, or of an acknowledgement, leaves an event whole or missing, and the mark before or after. | `go test -run 'TestOutboxCrashAtEachWriteStep|TestOutboxCrashDuringAck' ./src/client/internal/link/` passes. |
| 3 | A number is never given to two events, and a failed folder sync never lets an event be replaced. | `go test -run 'TestOutboxNeverReusesNumbers|TestAckDeletesOnlyAfterMarkAtEveryRemoval|TestConcurrentAppendsAreUnique|TestAppendAfterDirectorySyncErrorPreservesEvent|TestAckAfterDirectorySyncError' ./src/client/internal/link/` passes. |
| 4 | An acknowledgement or a number from the server never deletes an event that was not sent. | `go test -run 'TestAckRefusesNumberNotHandedOut|TestReconcileCases|TestNewOutboxKeepsEventsBeforeFirstReconcile' ./src/client/internal/link/` passes. |
| 5 | A damaged outbox is left untouched. | `go test -run 'TestRecoverChangesNothingWhenDamaged|TestAckedChecksumCatchesDigitChange|TestRecoverRefusesMissingMark' ./src/client/internal/link/` passes. |
| 6 | A full outbox refuses and loses nothing. | `go test -run 'TestAppendRefusesWhenFull|TestAppendRefusesAtSequenceCeiling' ./src/client/internal/link/` passes. |
| 7 | A command that arrives twice runs once and gets its first result. | `go test -run 'TestSecondArrivalReplaysResult|TestConcurrentBeginRunsOnce|TestReplayTruthTable' ./src/client/internal/link/` passes. |
| 8 | No command runs twice across a kill, a damaged receipt, a failed folder sync or a clock set back. | `go test -run 'TestReceiptsKillNeverRunsTwice|TestReceiptsCrashAtEachWriteStep|TestOpenMarksStartedAsInterrupted|TestDamagedReceiptIsNeverPruned|TestValidJSONWithBadContentIsDamaged|TestBeginAfterDirectorySyncError|TestClockHighWaterSurvivesRollback' ./src/client/internal/link/` passes. |
| 9 | A command past its expiry is not run, and a bad identifier never reaches the disk. | `go test -run 'TestBeginRefusesExpired|TestBeginExpiresAtTheInstant|TestBeginRejectsBadIDs' ./src/client/internal/link/` passes. |

**Design artifact:** none. This workstream has no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| An event is sent before it is on disk, and a crash gives its number to another event that the server then drops. | high | `Append` returns the number only after the safe write, proven by `TestAppendReturnsOnlyAfterDurable` and `TestOutboxKillKeepsEveryNumberedEvent`. |
| A folder sync fails after an event file is placed, and the next event replaces it. | high | Event files are linked, never renamed over, and the outbox stops after `ErrPlacedNotSynced` until it is opened again, proven by `TestAppendAfterDirectorySyncErrorPreservesEvent` (critic-r1 1). |
| A number from the server above the client's own deletes events that were never sent. | high | `Ack` refuses a number it never handed out, and `Reconcile` refuses a server number above it, proven by `TestAckRefusesNumberNotHandedOut` and `TestReconcileCases` (threat-r1 01). A lost outbox starts a new epoch instead of reusing numbers. |
| Acknowledged files are removed before the mark, and a crash lets numbers start again lower. | high | The mark is written first, proven by `TestAckDeletesOnlyAfterMarkAtEveryRemoval` and `TestOutboxNeverReusesNumbers`. |
| A changed mark deletes events that were never acknowledged. | high | The mark carries a checksum, and recovery checks everything before it deletes anything, proven by `TestAckedChecksumCatchesDigitChange` and `TestRecoverChangesNothingWhenDamaged` (threat-r1 02). |
| A command runs twice after a crash. | high | The `started` receipt is on disk before the command runs, and a restart turns it into `interrupted`, proven by `TestReceiptsKillNeverRunsTwice`. |
| A damaged receipt is pruned, and the command runs again. | high | A damaged receipt is pinned and never pruned, proven by `TestDamagedReceiptIsNeverPruned` (critic-r1 4). |
| An identifier from the server names a file outside the receipts folder. | high | Only the canonical form of a UUID is accepted, proven by `TestBeginRejectsBadIDs`. |
| The client's clock is set back, and an expired command runs. | medium | The receipts keep the highest time they have seen, proven by `TestClockHighWaterSurvivesRollback` (logic2-r1 11). A clock that runs behind the server's is a choice for the owner. |
| A long outage fills the outbox. | medium | The outbox refuses, and the producer keeps the event and tries again, as the section "What plan go-s3-client-link must do" says (architect-r1 6). The limits are options. |
| Two disk syncs for each event are too slow for wave 2. | low | Decision 16 keeps the interface, so wave 2 can change the storage behind it. |

**Verify:** the tests of items 6, 7 and 8, all under `-race`. Device check: none. The kill tests run on Linux, and phase 5 runs them on a Mac when one is available. The section "Post-build checks" covers a real loss of power.

**Rollout detail:** steps 3 and 4 of the rollout order.

</details>

<details><summary><b>What plan go-s3-client-link must do</b></summary>

This plan builds pieces that are safe only when their caller keeps a few rules.
Plan `go-s3-client-link` is that caller, so its planner reads this list before freezing the plan.
Each rule names the finding of round 1 that it answers.

**Events.**

1. A producer of an event keeps its event when `Append` refuses it with `ErrOutboxFull`, and tries again later, or it skips the event only when a newer event replaces it, as a newer heartbeat replaces an older one (architect-r1 6, critic-r1 11, threat-r1 12).
2. A scan of the harnesses counts as finished only when its report is in the outbox. A scan whose report was refused runs again.
3. One sender per machine reads the outbox and sends events in order. The number on the wire is `Event.Seq`, and no other counter exists.
4. The payload of an event is a versioned report body that carries the time the producer made it. A repeat of the same content under a new number is allowed, and the server applies such reports so that a second copy changes nothing (logic2-r1 9).
5. After a failed `Append` with `ErrOutboxFailed`, the producer closes the outbox, opens it again, and treats its event as possibly stored, so that it never makes a second copy in a hurry (architect-r1 2).
6. On each connection, after both ends have proven themselves, the client tells the server its epoch, and the server answers with its last applied number for that epoch. The client then calls `Reconcile` with that number, and sends from the first event that is still pending. `ErrServerBehind` and `ErrServerAhead` close the connection, log one sentence, and retry later (architect-r1 3, threat-r1 01).
7. When `OpenOutbox` returns `ErrOutboxCorrupt`, `run` exits with one sentence that names the damaged file, until the owner rules on the choice in the section "For the owner" (logic2-r1 6).

**Commands.**

8. A command runs only after `Begin` returns `DecisionRun`. Any error from `Begin` means the command does not run.
9. The client calls `Finish` for every command it ran, before it restarts or exits for any reason it controls (logic-r1 8).
10. A replay sends the stored outcome, `done`, `failed`, `interrupted` or `expired`, with the attempt of the command that just arrived. The contract has no outcome `interrupted` today, so the result file asks the Contract team for it (logic2-r1 3).
11. The server never changes the expiry of a command under the same identifier, and it sends a new identifier when it wants work done again. The result file asks the Server team to keep this rule.

**Joining and the state folder.**

12. `join` opens the state folder with `state.Open` and holds it until it ends. `run` does the same (threat-r1 04).
13. `join` refuses a folder that already holds `server.json`. A new enrollment needs a new folder (logic2-r1 2).
14. `join` calls `enroll.Load` first. Only on `ErrKeyMissing` does it call `enroll.Create`. On `ErrPlacedNotSynced` from `Create`, it calls `Load`. It never makes a second key for one folder (architect-r1 7, logic-r1 7, threat-r1 09).
15. `join` writes `server.json` with `state.WriteAtomic` last, only after the server confirmed the join. A key without `server.json` means a join that did not finish, and the next `join` reuses that key with the same token, which the draft contract allows.
16. `run` refuses to start when `server.json` is missing or not valid, with one sentence that says to run `join`.
17. `run` logs the resolved state folder and the resolved folder for clones once at start (logic2-r1 12).

**The installer.**

18. Plan `go-s3-stack-client` sets `LGTM_CLIENT_STATE_DIR` in the service file it installs, so that the service and a person who runs `lgtm-client` by hand use the same folder (logic2-r1 12).

</details>

<details><summary><b>Post-build checks</b></summary>

These checks need the built code, so they run in phase 5 of the build loop, or in sprint 3 where a row says so.
Each row gives the scenario and the result that must be seen.

| # | Check | Scenario | Expected result | From |
|---|---|---|---|---|
| 1 | A real loss of power on Linux | On a test machine with the client's state folder on its own disk, a loop appends events and runs commands. The machine's power is cut 10 times at random moments. | After each restart, `OpenOutbox` and `OpenReceipts` succeed or name a damaged file. Every event whose number was printed is pending or acknowledged, and no command appears twice in the file of effects. | architect-r1 1, critic-r1 2, logic-r1 4 |
| 2 | A real loss of power on macOS | The same as check 1, on a Mac with the folder on its internal disk. | The same as check 1. A gap is logged when no Mac is available. | critic-r1 2 |
| 3 | The order after a reconnection | In sprint 3, the server is stopped while the client appends 50 events, and then started again. | The server applies all 50 events once and in order, and `Reconcile` is called once on the new connection. | architect-r1 3, threat-r1 01 |
| 4 | A join that stops half-way | In sprint 3, the client is killed after `machine.key` is written and before `server.json`. `join` then runs again with the same token. | The second `join` reuses the key, and the server shows one machine, not two. | architect-r1 7, logic-r1 7 |
| 5 | The owner checks as the superuser | A person runs the tests of items 2 and 3 as the superuser, with a key file and a state folder owned by another user. | `Open` returns `ErrFolderNotOwned`, and `Load` returns `ErrKeyNotOwned`. | threat-r1 04, threat-r1 11 |
| 6 | The scan under load | The scan runs while the machine runs a build that keeps every core busy. | `Scan` returns within `Timeout` plus `WaitDelay` plus 1 second, and each installed harness is found. | architect-r1 9 |
| 7 | The real version lines | On a machine with each of the four harnesses installed, a person runs `Scan`. | Each installed harness is found with its version. A harness whose line does not match is recorded, and its pattern is fixed. | threat-r1 10 |
| 8 | The report of the harnesses on the wire | In sprint 3, the harness report reaches the server. | The report carries only the harness name and the version. It carries no path and no problem text. | architect-r1 9 |
| 9 | The count of effects of a command | In sprint 3, a command with a visible effect, such as a file it writes, is sent twice, and the client is killed once while it runs. | The effect happens at most once, and the second arrival gets the stored outcome. | logic-r1 2, critic-r1 4 |
| 10 | Samples from macOS | On a Mac, a person saves the raw bytes of `vm.loadavg`, `hw.memsize` and `kern.memorystatus_level` as fixtures for item 4. | The decoders of item 4 give the values that `/usr/sbin/sysctl -n` prints for the same names. | critic-r1 9 |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe:

1. The six work items of the brief (`plan.md` of 2026-11-25).
2. The four points the brief asks the planner to settle.
3. The four points of the task file `.foundry/go-rewrite/tasks/plan-go-s1-client-local.md` under "Specific to this plan".
4. The statements of the design that this plan builds on, as listed in `scouting.md`: D2, D7, D8, D16, D19, R1, R6, R9, R10, C7, C10, C11, and the sections "Joining" step 3, "Commands go down", "Events come up", "What travels in wave 1", "Knowing whether a machine is there" and "The client in wave 1".
5. Point 4 of the epic's section "What the teams must agree on".
6. The findings of review round 1, listed in `attack/round-1-findings.md`.

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
| Round 1 findings fixed in this plan's items | WS-1 to WS-4, at the places the register's patch list names | mapped |
| Round 1 findings that the caller must keep | Fold "What plan go-s3-client-link must do" | mapped |
| Round 1 findings that need the built code | Fold "Post-build checks" | mapped |
| Round 1 findings for the owner | Section "For the owner"; decisions 48 and 49 | mapped |

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
| 2. The state folder and safe writes | backend | client/backend | medium | A known method of writing in two small files, made risky by ordering and covered by named crash tests. Round 1 added four small checks, each with its own test. | no | `go.mod` |
| 3. The key of the machine | backend | client/backend | medium | The standard library does the cryptography. The work is file handling with one test for each refusal. | no | 2 |
| 4. The facts and the load | backend | client/backend | medium | Two operating systems and a binary decoder, all pure functions with fixtures. | no | `go.mod` |
| 5. The harness scan | backend | client/backend | medium | Child processes under a time limit, a known pattern with clear tests. Round 1 added one small file that checks where a program lives. | no | `go.mod` |
| 6. Event files, the epoch and the mark | backend | client/backend | medium | A small binary format, two small text files and one recovery function, with a fixture for each crash state. | no | 2 |
| 7. The outbox queue | backend | client/backend | medium | Numbers, limits, one lock, one failed state and one check of the server's number on top of item 6, with a kill test. Round 2 should say whether this is now high, and if so it splits into the queue and the reconciliation. | no | 6 |
| 8. Receipts of commands | backend | client/backend | medium | One record with five states on top of item 2, a clock that never goes back, and a kill test. Round 2 should say whether this is now high. | no | 2 |

No unit is rated high. The brief rated items 2 and 5 high, and both were split by decision 7.
No unit changes an entry point that outsiders can reach, so no unit needs an attack pass.
The scope file `.plans/scopes/client/backend/scope.json` exists in the checkout, and every unit uses it (architect-r1 11).

Load: each unit has one test writer, one builder and one code reviewer, and the reader of the third vendor beside the code review. The five risky units (2, 3, 6, 7 and 8) add one break reviewer each. That is 8 test writers, 8 builders, 8 code reviewers, 8 readers and 5 break reviewers, 37 launches in all. With builders alternating between the two main vendors, one of them takes 15 launches and the other 14, and the third vendor takes the 8 reader seats. The orchestrator checks these counts against each vendor's weekly meter at the start of sprint 1, as the epic says. If one vendor would run out, the units of WS-3, which share nothing with the others, are the first to move later in the sprint.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 0 | none yet | The planner stopped after the self-check, as the shared rules for planners say. The orchestrator runs the review rounds with other vendors. |
| 1 | 57 findings from five reviewers: 18 blockers, 30 majors and 9 minors, in 26 groups, 11 of which hold more than one finding. | Every finding has an outcome in `attack/round-1-findings.md`. The promise that buffered events arrive once and in order (C11) is stuck, with 55 of 116 points. Three mechanisms are new and marked UNREVIEWED MECHANISM: the epoch of the outbox, the reconciliation with the server's number, and the rule for a trusted program. Two choices went to the owner. The next round attacks only the patch list of the register. |

Known gaps from the self-check, for the reviewers:

- Gate 1: row 5 of `decisions.md` is open. It is the owner's decision on the cut of wave 1 into sprints, and the planner cannot close it. Decision 34 records that the plan builds on the cut as written.
- Gate 7: round 1 ran, and round 2 has not.
- Gate 10: the remaining weekly allowance of each vendor is not known to the planner.
- The epoch and the outcome `interrupted` need fields in the contract that the Contract team has not yet accepted.
- The owner check of item 3 has no test, because a test cannot make a file owned by another user without the rights of the superuser.
- macOS code is compiled and vetted on Linux, but its tests run only when a Mac is available in phase 5.

</details>
