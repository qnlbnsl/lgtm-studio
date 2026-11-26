# Review of the built code of plan go-s1-client-local, items 1, 2 and 3 (post-build, reviewer a)

- Task identifier: postbuild-go-s1-client-local-a
- Nonce: f0496f
- Reviewer model and effort: claude-sonnet-5-5, high. I did not build this code and did not write its tests.
- Code read: `git diff 20d5c4d9..8e22c110 -- src/client` for the code, and the frozen tests directly in their folders, because the diff shows only the implementation.

## Verdict: SHIP

The code of items 1, 2 and 3 does what the plan says, and I found no blocker and no major defect.

The top finding is `post-a 1`: `state.Open` fails with a bare `permission denied` when an ancestor of the state folder can be searched but not read, because it syncs every ancestor by opening it for reading.

## What I ran

All commands ran from the repository root on the branch `sprint-0`, as an ordinary user.

| Command | Result |
|---|---|
| `go test -race -count=1 ./src/client/cmd/... ./src/client/internal/config/... ./src/client/internal/state/... ./src/client/internal/enroll/...` | All four packages print `ok`. |
| `go vet` of the same packages for `linux` and `darwin`, each with `amd64` and `arm64`, `CGO_ENABLED=0` | Exit 0 for all four. |
| `gofmt -l` of the same packages | Prints nothing. |
| The same tests as the superuser, in an `alpine:latest` container that I started and that was removed on exit | Every test passes, and my two extra owner tests pass (see the next section). |

## The places no reviewer read before the build

The plan marks no place of items 1, 2 or 3 as `UNREVIEWED FIX`.
All seven places that `attack/round-4-findings.md` lists under "Not reviewed before the build" belong to item 6 or to plan `go-s3-client-link`, and another reviewer owns them.
Two post-build checks of the plan belong to my items.

- **Row 1, a real loss of power.** I did not run it, because it needs a machine whose power I can cut ten times. The kill tests prove only the order of the steps, as the plan says.
- **Row 2, the owner checks as the superuser.** I ran it for items 2 and 3, and it passes. As the superuser in my own container, with files and folders owned by another user: `Open` of a state folder owned by user 1234 returns `ErrFolderNotOwned`, `Open` below a parent owned by user 1234 returns `ErrParentUnsafe` and creates nothing, and `Load` of a key owned by user 1234 with mode `0600` returns `ErrKeyNotOwned`. The part for item 6 (`OpenStore` returns `ErrStoreUnsafe`) is not mine.

## The code against the plan

Every public name, every error and every refusal of items 1, 2 and 3 that the plan lists exists and behaves as the plan says.
I read `main.go`, `config.go`, `folder.go`, `atomic.go`, `testhook.go` and `credential.go` against the "Behaviour" lists.
The differences are in the findings: `post-a 5` is the only one where the code accepts more than the plan allows.

## The mutations

The builder reports that it ran the seven mutations of item 2 and printed the red test of each. I ran them again in scratch copies made with `mktemp -d`, and every one turned the named test red, as did the plan's five mutations of item 3. These are the mutations of the plan that I could reproduce:

| Mutation | Test that went red |
|---|---|
| Write the final file in place | `TestWriteAtomicCrashAtEachStep/write-temp-created` |
| Remove the sync of the temporary file | `TestWriteAtomicSyncsBeforeRename` |
| Remove the check of the folder's bits | `TestOpenRefusesExposedFolder` |
| Remove the `flock` | `TestOpenRefusesSecondHolder` |
| Rename in `CreateExclusive` | `TestCreateExclusiveNeverReplaces` |
| Sync only the parent | `TestOpenSyncsWholeChain` |
| Return a plain error after placement | `TestWriteReportsPlacedNotSynced` |
| Remove the check of the key's bits in `Load` | `TestLoadRefusesExposedKey` |
| Write the temporary file with mode `0644` | `TestCreateWritesPrivateKeyFile` |
| Open without `O_NOFOLLOW` | `TestLoadRefusesNonRegular` |
| Remove `String`, or remove `GoString` | `TestCredentialNeverPrintsKey` (the build fails first, so this proves little) |
| Use `state.WriteAtomic` in `Create` | `TestCreateRefusesExistingKey` |

One claim of the plan does not hold: removing only `Format` leaves every test green. That is harmless, because `String` still answers every verb, but the plan's sentence "removing `String`, `GoString` or `Format` makes `TestCredentialNeverPrintsKey` fail" is wrong for `Format` (`post-a 4`).

## Findings

### post-a 1, MINOR: `Open` fails when an ancestor can be searched but not read

- **Statement:** D19 and the plan's item 2 text, "Before it returns, `Open` syncs every folder of the resolved chain".
- **Evidence:** `src/client/internal/state/folder.go:123` loops over every folder from `/` to the state folder, and `src/client/internal/state/atomic.go:135` syncs each with `os.Open(dir)`, which needs the read bit.
- **Failure scenario:** I ran this as user 1000 in a container. `/srv-probe` was owned by the superuser with mode `0711`, and `svc` below it was owned by user 1000 with mode `0700`. `Open("/srv-probe/svc/state")` returned `open /srv-probe: permission denied`. A person who sets `LGTM_CLIENT_STATE_DIR` below a shared folder of mode `0711`, such as a `/home` on a shared server, gets an error that does not name the state folder or the rule.
- **Fix:** Wrap the error as `state: cannot sync <path> of the chain of <dir>: …`, and either skip the sync of an ancestor that the user does not own and cannot read, or state in the plan that every ancestor must be readable.

### post-a 2, MINOR: no test fails when `RemoveTemps` runs before the lock

- **Statement:** Plan item 2, "Under the lock it calls `RemoveTemps`". Design D19, one client at a time.
- **Evidence:** `folder.go:120` is inside `lockFolder`, after the `flock`. `TestOpenCreatesPrivateFolders` checks only that temporary names are gone after a later `Open`.
- **Failure scenario:** I moved `RemoveTemps(dir)` above the `OpenFile` of `lock`. Every test of the package passed. With that change, a second client that fails with `ErrFolderLocked` has already deleted the temporary files that the running client is writing, so the running client's `rename` or `link` fails.
- **Fix:** Add a case to `TestOpenRefusesSecondHolder`: while a child holds the folder, plant `.tmp-held` in it, expect `ErrFolderLocked` from `Open`, and expect the file to still exist.

### post-a 3, MINOR: no test pins what `Create` returns after `ErrPlacedNotSynced`

- **Statement:** Plan item 3, "On `ErrPlacedNotSynced`, `Create` returns that error and no credential".
- **Evidence:** `credential.go:69` returns the error. No test in `credential_test.go` uses `state.SetSyncFolderForTest`.
- **Failure scenario:** I changed `Create` to ignore `ErrPlacedNotSynced` and return the credential. All tests passed. The caller would then print a join as done with a key that a loss of power can remove, which is the case rule 8 of the fold "What plan go-s3-client-link must do" exists for.
- **Fix:** Add `TestCreateReportsPlacedNotSynced`: with the folder sync failing, `Create` returns an error for which `errors.Is(err, state.ErrPlacedNotSynced)` is true and a nil credential, and the file holds a loadable key.

### post-a 4, MINOR: three checks of `Load` and `Open` have no test, and one sentence of the plan is wrong

- **Statement:** Plan item 3, "`Load` reads at most 4,097 bytes", "exactly one PEM block ... holding an Ed25519 key", and item 2, "`Open` opens `lock` with `O_NOFOLLOW`". Plan item 3 "Risky" line for `Format`.
- **Evidence:** `credential.go:109` (the read limit), `credential.go:138` (the check that nothing follows the encoded value inside the block), `folder.go:95` (`O_NOFOLLOW`). The tests contain no case for any of them.
- **Failure scenario:** I removed each in a scratch copy, and every test of the package stayed green. Without the limit, a state folder whose `machine.key` is a file of many gigabytes makes `Load` read all of it into memory before it refuses. Without `O_NOFOLLOW`, a `lock` that is a link is followed. The test `TestCredentialNeverPrintsKey` also stays green when only `Format` is removed.
- **Fix:** Add a key file of 10 mebibytes that must give `ErrKeyInvalid` while the test checks the heap with `testing.AllocsPerRun` or with the size read, a valid key with one byte appended inside the DER, a `lock` that is a link to a file, and correct the sentence of the plan to name `String` and `GoString`.

### post-a 5, MINOR: `Load` accepts a key file that the plan says is invalid

- **Statement:** Plan item 3 and the layout table, "One PEM block of type `PRIVATE KEY` ... and at most one newline after it".
- **Evidence:** `credential.go:125` accepts `\r\n` after the block and line ends of `\r\n` throughout, and `credential.go:132` ignores `block.Headers`. I ran `Load` on three files in a scratch copy: a key whose lines all end in `\r\n`, a key with a header line `X-Note: hi`, and a key without a newline. All three loaded with no error.
- **Failure scenario:** A file that was copied through a tool that changes line ends, or one with a header that another program added, loads as the machine key. This does not leak anything, but the plan's rule of a fixed format is not kept, and the next reader of the key file (a second program) may parse it differently.
- **Fix:** Refuse a block with headers and any byte `\r`, or change the plan's sentence to say that these forms are accepted, and add a test for either choice.

### post-a 6, MINOR: `Paths()` on a nil `*Folder` panics

- **Statement:** Plan item 2, "A claim on a `nil` or zero folder ... returns `ErrFolderClosed`".
- **Evidence:** `folder.go:203`, `func (f *Folder) Paths() Paths { return f.paths }`. I called it on a nil pointer and got a nil pointer dereference. `Claim` and `Close` handle a nil folder.
- **Failure scenario:** Code of plan `go-s3-client-link` that keeps a `*Folder` in a struct and calls `Paths()` after an `Open` that failed crashes the client where `Claim` would have returned an error.
- **Fix:** Return `Paths{}` when `f == nil`, and add the case to `TestClaimAllowsOneStoreEach`.

### post-a 7, MINOR: a placed file can be reported as a plain failure

- **Statement:** Plan item 2, "A failure before `placed` changes nothing", and item 3, `Create` returns `ErrKeyExists` for an existing file.
- **Evidence:** `atomic.go:66-78`. After `os.Link` placed the file, a failure of `os.Remove(tempPath)` is returned as a plain error when the folder sync succeeds. `Create` at `credential.go:69` then returns that plain error and no credential, although the key file is in place.
- **Failure scenario:** The disk or a security module refuses the remove of the temporary name after the link. `join` sees a plain error and stops, the key stays, and the next `join` gets `ErrKeyExists` with no way to know whether the first run finished. The cause is rare, and the state heals once `Open` removes the temporary file.
- **Fix:** Return `ErrPlacedNotSynced`, or a new error that wraps it, when `removeErr` is set after `placed`, so that the caller knows the file is in place.

### post-a 8, MINOR: `WriteAtomic` can replace the lock file

- **Statement:** D19 (one client at a time) and the layout table, where `lock` holds an exclusive `flock` while a client uses the folder.
- **Evidence:** `atomic.go:29-34` refuses only names that are empty, `.`, `..`, hold a separator or start with `.tmp-`. A name such as `lock` passes, and `os.Rename` over `lock` gives it a new inode.
- **Failure scenario:** A call `WriteAtomic(dir, "lock", …)` leaves the first client holding a lock on a file that no longer has a name, and a second `Open` creates a new `lock` and gets its own lock. Two clients then hold one folder. No code of the plan makes this call today, but the API invites it.
- **Fix:** Add `LockFile` and `StoreFile` to the names that `WriteAtomic` refuses with `ErrBadName`, or say in the plan that callers must not use them.

## What I checked and found clean

- The key never reaches a printed text: the key sits behind a pointer and in a closure, the printing methods return the fingerprint, and `Load` and `Create` put only fingerprints and file names in their errors.
- `Load` cannot be blocked by a named pipe: it opens with `O_NONBLOCK`, checks the open file with `Stat`, and refuses a pipe, a link and a folder with `ErrKeyNotRegular`.
- Every error path of `writeAtomic` removes its temporary file, and the file descriptors are closed on every path that I followed.
- `LoadSettings` refuses every listed case. It also refuses `null` values, trailing text after the object, and a missing `version`, which the plan does not list but which are correct.
- Items 1, 2 and 3 import only the standard library, and `go vet` for `darwin` passes. The post-build check for a Mac (`F_FULLFSYNC`) is open, as the plan says.

## Grade: 8 of 10

The code is complete, careful and tied to the plan.
Two points are lost for the three places where a wrong implementation passes every test (`post-a 2`, `post-a 3` and `post-a 4`), and none for the other findings, which are small.
