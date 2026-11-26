# Post-build review B: items 4, 5 and 6 of the plan go-s1-client-local

## Verdict

REVISE, and only for the tests. The code of the three items does what the plan says, and every risk I could run came out right.

The top finding is post-b 1: the fixture that should prove that `Finish` keeps old started receipts is itself an invalid receipt, so a `Finish` that deletes valid started receipts passes every test.

## What I ran

- The tests of `sysinfo`, `harness` and `link` pass under the race detector.
- `go vet` passes for linux and darwin on amd64 and arm64, and `gofmt` reports nothing.
- `scripts/check-go.sh` ends with every package ok (post-build row 10). The `go.mod` diff adds `go.etcd.io/bbolt v1.4.3` and `golang.org/x/sys v0.29.0` as an indirect requirement, and nothing else.
- The frozen test files have the hashes in `.foundry/go-rewrite/frozen-s1client-b.sha256`. The reordered `TestOpenStoreCreatesSyncedPrivateFile` differs from its first version in the position of one line (`f2, _ := newState(t)`), and it passes.
- `Scan` on this machine finds Claude, Codex and Cursor with their real versions, and reports Gemini as not found (post-build row 3, for three of the four harnesses).
- As the superuser in a container that I started and that is gone, `OpenStore` on a store file whose owner I changed to another user returns `ErrStoreUnsafe` (row 2). The same container ran the unsafe-file tests of the link and state packages, and they pass.
- A mode 000 store file and a mode 400 store file gave the answer described in post-b 5.
- An empty and a nil payload are appended, survive a reopen, and are read back (no finding).
- `check-go.sh` is the only check that touched the repository checkout. Every other experiment ran in a scratch copy made with `mktemp -d`, which I left in place.

## Places no reviewer read before the build

| Place | What I did | Result |
|---|---|---|
| The check at open, with the read-only handle and the size test (rows 11 and 12 of round 4) | Ran `TestOpenStoreRefusesDamagedFile` with bbolt v1.4.3, and removed the walk of event keys in a scratch copy. | The test fails when the walk is removed, so it checks the walk. I did not run v1.5.0. |
| Receipts that decode but are invalid (row 12) | Read `readReceipt` and `Begin`, and ran `TestStartedFromEarlierRunIsInterrupted` against a mutation that runs such a command. | The mutation fails the test. The answer is `DecisionInterrupted` and the value is not touched. |
| Pruning by age in `Finish` | Mutated the prune rule. | Not caught, see post-b 1. |
| The module file (row 10) | Ran `check-go.sh` and read the diff. | Passes, nothing new beyond bbolt and `x/sys`. |
| The creation of the file with a link and a folder sync | Ran the crash tests and the sync mutation. | The writable open is checked for sync. The creation open is not, see post-b 4. |

## Rows I could not run

Rows 1 and 4 need real power loss and a failing disk. Rows 5, 6, 7, 9 and 13 belong to sprint 3. Row 8 needs the command `run` of item 1 against a built store. Row 11 with bbolt v1.5.0 was not tried. The macOS decoders ran only through the cross vet, because there is no Mac here, and the test `TestReadLoadOnMacMatchesSystem` was not run.

## The code against the plan

Each "Done when" row of items 4, 5 and 6 holds. The plan text and the code agree on the open order (read-only check, then writable open), on `Begin` and `Finish`, on the clean environment and the shared deadline of `Scan`, and on the padding of the macOS decoders. I found no difference that the builder's result file does not explain.

## Mutations tried in a scratch copy

| Mutation | Result |
|---|---|
| Child folder is not the home | Caught by `TestScanGivesOnlyVersionAndCleanEnvironment`. |
| `LANG=C` removed from the environment | Caught by the same test. |
| Exit status ignored in `readVersion` | Caught by `TestScanReportsFailureAndNoVersion`. |
| The scan made sequential | Passes, and it should: the plan asks for a shared deadline, not for parallelism. |
| The four-kibibyte overflow flag ignored | Passes. The patterns are anchored, so a longer line can never match and the flag is redundant. I do not count this. |
| `NoSync` on the writable open | Caught by `TestOpenStoreCreatesSyncedPrivateFile`. |
| `Ack` accepts the next number | Caught by `TestAckRefusesAhead`. |
| A failed commit does not fail the store | Caught by `TestCommitErrorFailsStore` and `TestFailureStopsWaitingWriter`. |
| The walk of event keys at open skipped | Caught by `TestOpenStoreRefusesDamagedFile`. |
| A command runs although a started receipt of an earlier run exists | Caught by `TestStartedFromEarlierRunIsInterrupted`. |
| `Finish` prunes started receipts | **Not caught**, post-b 1. |
| The owner check removed from `checkStoreFile` | **Not caught**, post-b 2. |
| `Read` returns the bbolt slice without a copy | **Not caught**, post-b 3. |
| `NoSync` on the file made at creation | **Not caught**, post-b 4. |

## Findings

### post-b 1, MAJOR: the test of pruning cannot see a started receipt that is wrongly pruned

- **Statement.** `TestFinishPrunesByAge` is the test for the rule "only finished receipts are pruned", and its fixture for a started receipt is not a valid started receipt.
- **Evidence.** `receipts_test.go:16-19` builds every fixture with `"result":%q`, so `receiptJSON(..., "started", nil)` at `receipts_test.go:259` contains `"result":""`. `readReceipt` at `receipts.go:46-47` accepts a started receipt only when `len(r.Result) == 0`, and the raw text `""` has length 2. The fixture is therefore invalid, and `pruneReceipts` at `receipts.go:154` skips it before it looks at the state. With the state test removed (`valid && r.ExpiresAt.Before(cutoff)`), all link tests pass. The builder's result file names this gap.
- **Failure scenario.** A later change prunes every valid receipt past its age. A command whose process died is then forgotten 25 hours after its expiry, and the server's next attempt gets `EXPIRED` instead of `INTERRUPTED`, so the record that the command may have run is gone.
- **Fix.** Write the started fixture as `{"version":1,"expires_at":"...","state":"started"}` with no result, and add an assertion that it is valid by reading it with `Begin` after the prune (decision `INTERRUPTED`).

### post-b 2, MAJOR: no test makes a store file owned by another user

- **Statement.** The owner test in `checkStoreFile` is never exercised by a failing case.
- **Evidence.** `store.go:236-239` compares the owner to the effective user, and `store_test.go:334-370` (`TestOpenStoreRefusesUnsafeFile`) has no `Chown` and no second user. With the owner comparison removed in a scratch copy, the whole link package passes. The plan's row 2 says a person runs the tests as the superuser with a file owned by another user, and the tests as written do not create that file, so the row can only be passed by hand. I ran it by hand in a container and it works (`ErrStoreUnsafe`).
- **Failure scenario.** A refactor drops the owner comparison. A store file that another user owns and keeps at mode 600 is opened for writing as root, and nothing fails in CI.
- **Fix.** Add a test that runs only when the effective user is the superuser, creates a store, calls `os.Chown` to another user, and expects `ErrStoreUnsafe` from `OpenStore`. Run it in the container step of the checks. This protects the plan's rule for item 6 that a store file must be private and owned by the running user.

### post-b 3, MINOR: nothing checks that `Read` copies the payload

- **Statement.** `Read` returns copies made with `bytes.Clone` inside the view, and no test would notice if it returned the bbolt slice.
- **Evidence.** `outbox.go`, `Read`, builds `Event{Payload: bytes.Clone(payload)}`. With `payload` in its place, every test passes.
- **Failure scenario.** A bbolt slice is valid only inside its transaction. A caller that keeps the slice while a later `Append` grows and remaps the file reads freed memory, and the program faults.
- **Fix.** Add a test that reads, appends enough events to grow the file, and compares the earlier payload. A test that changes the returned bytes and reads again also works.

### post-b 4, MINOR: nothing checks that the file at creation is opened with sync

- **Statement.** The writable open is checked for `NoSync`, and the open that creates the file is not.
- **Evidence.** The test at `store_test.go:290` calls `s.noSync()` after `CreateStore`, which looks at the store that is returned and not at the handle used on the temporary file (`store.go:175`). With `NoSync` set on that open, all tests pass.
- **Failure scenario.** After a power loss, `client.db` exists but its first pages are not on disk. The check at open then reports damage on a joined machine, and its outbox is lost. Row 1 would find it, and row 1 cannot run in a build.
- **Fix.** Pass the options through the same function that `noSync` reads, or add a hook that records the options of the creation open and assert it.

### post-b 5, MINOR: a store file that cannot be opened for permission reasons is reported as damaged

- **Statement.** `storeOpenError` turns every error that is neither a timeout, a symbolic link nor a known damage into `ErrStoreDamaged`.
- **Evidence.** I made a store with one event and set its mode to 000, and then to 400. `OpenStore` returned `client.db: link: store is damaged: open ...: permission denied` in both cases, and `errors.Is(err, ErrStoreDamaged)` is true.
- **Failure scenario.** A read-only disk, a wrong mode after a restore, or an input and output error gives the sentence "store is damaged". The owner follows the advice for damage and removes `client.db`, and the unsent events and the receipts are lost, although the file is healthy.
- **Fix.** Map permission, read-only and input-output errors to an error that names the cause and keeps `ErrStoreDamaged` for the failed checks of the content.

## Not findings, for the record

- A Linux machine in a container with a quota counts all host cores in `Cores`. The plan names `runtime.NumCPU()`, so this is as designed.
- The scan decides that a candidate exists with `Lstat`. This is what the plan says, and the later `exec` fails safely with `ProblemFailed`.
- `Finish` can prune the receipt it has just written when the command ran for more than a day past its own expiry. That is rare, the answer is then `EXPIRED`, and it is safe.

## Grade

7 of 10. The code is clean, small, and does what the plan says, and the experiments that I could run all came out right. The grade is held down by four mutations that the frozen tests accept, one of which (post-b 1) is on a risk that the plan names.
