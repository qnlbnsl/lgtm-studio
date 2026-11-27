# REVISE

The new object scan can silently skip entries on filesystems that omit directory-entry types, leaving Git with a store the client checked only in part.

The cross-vendor rule is met for the repair on `sprint-0`. Claude workers wrote the repair tests and implementation. Codex workers wrote the original item 4 code, and a separate Codex worker reviewed the repair at `3a9e915c`. The task identifier is `postbuild-go-s4-client-folders-4b` and the nonce is `98f034`. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s4-client-folders-4b.md#L1) · [Builder handoff](../../../../.foundry/go-rewrite/results/s4-client-4r-build.md#L1)

## Findings

### post-4b 1 · MAJOR · plan

By reading, the new scan can miss entries even when the object store is stable. It gives `os.NewFile` an empty name and then calls `ReadDir`. When the filesystem returns an unknown entry type, Go 1.26 resolves the type with a metadata lookup formed from the file's name and the entry name. Here that lookup is at the filesystem root. Go silently skips an absent entry there. These branches are in the standard library at `os/file_unix.go:468`, `os/dir_unix.go:141` and `os/dir_darwin.go:91`. A filesystem reporting an unknown type for the repository's `pack` directory can therefore cause the scanner to omit that entire subtree. Read entry names and classify each one with a descriptor-relative, no-follow stat. Add coverage for a filesystem that omits entry types or exercise the classification independently of the directory record. [Object scan](../../../../src/client/internal/folders/checkout.go#L1034) · [Plan](../plan.md#L144)

### post-4b 2 · MAJOR · plan

By reading, finding post-4 2 remains open for changes after the scan. The check closes its directory descriptors and the snapshot records the live source path for Git. The final consistency check revisits copied sources only. If the object store changes between the scan and Git's object lookup, Git can see files and alternate-store configuration that the scan never approved. The same window extends through the status commands run by item 5 after capture returns. Give Git a private object view whose inputs stay inside the approved store throughout capture and use, or enforce that boundary at each object access. Another scan after Git runs would detect some changes after the read. [Alternate path](../../../../src/client/internal/folders/checkout.go#L922) · [Descriptor lifetime](../../../../src/client/internal/folders/checkout.go#L1005) · [Final check and handoff](../../../../src/client/internal/folders/checkout.go#L1263) · [Builder handoff](../../../../.foundry/go-rewrite/results/s4-client-4r-build.md#L176) · [Plan](../plan.md#L147)

### post-4b 3 · MAJOR · plan

By reading, finding post-4 1 remains open when checkout discovery chooses its Git directory. It follows a directory link at `.git` and trusts a Git directory named by a `.git` file when `commondir` is absent. The chosen directory then becomes an approved root for configuration, references and objects. A checkout marker that points at unrelated repository metadata can therefore cause that metadata to be read and reported as the checkout's own. Validate the relationship before admitting an external metadata root. Keep the existing reciprocal check for linked worktrees, validate populated submodules against their superproject and require an independently established association for any supported separate Git directory. [Root selection](../../../../src/client/internal/folders/checkout.go#L247) · [Scope of reciprocal validation](../../../../src/client/internal/folders/checkout.go#L271) · [Configuration read](../../../../src/client/internal/folders/checkout.go#L881) · [Builder handoff](../../../../.foundry/go-rewrite/results/s4-client-4r-build.md#L175) · [Plan](../plan.md#L144)

### post-4b 4 · MINOR · plan

Confirmed live, discovery can turn a broken symbolic `HEAD` link into a valid branch in the private view. The reader accepts a raw link target with trailing whitespace, then serializes it as a textual reference whose whitespace is stripped during parsing. With a trailing space after the name of an existing branch, native Git exited 128 without an object identifier. Discovery returned success and reported that branch. Validate the exact link target as a branch reference before serializing it and add a native broken-link regression case. [Link conversion](../../../../src/client/internal/folders/checkout.go#L724) · [Reference normalization](../../../../src/client/internal/folders/checkout.go#L1071) · [Frozen symbolic-reference cases](../../../../src/client/internal/folders/checkout_review_test.go#L453) · [Plan](../plan.md#L145)

## Private snapshot and item 5 handoff

The plan marks the private snapshot as the previously unreviewed mechanism. The round-one register assigns the snapshot to item 4, and the fold records zero findings separately deferred to post-build review. The native read-only scenario applies here. [Plan mechanism](../plan.md#L147) · [Native scenario](../plan.md#L269) · [Register](round-1-findings.md#L38) · [Fold result](../../../../.foundry/go-rewrite/results/fold-go-s4-client-folders-r1.md#L9)

Confirmed live on Linux, the split-index and sparse fixtures passed. The configuration and index replacement fixtures allowed a coherent view or a fixed refusal and cleanup. The missing-object and executable-configuration fixtures observed zero requests and no execution marker. A scratch probe cancelled after it observed the private directory and confirmed a fixed refusal, an empty temporary directory and unchanged source files. The planned tests also checked cleanup after success and other refusals. [Linked-worktree fixture](../../../../src/client/internal/folders/checkout_test.go#L480) · [Sparse fixture](../../../../src/client/internal/folders/checkout_test.go#L563) · [Marker and request fixtures](../../../../src/client/internal/folders/checkout_test.go#L589) · [Replacement fixtures](../../../../src/client/internal/folders/checkout_test.go#L693)

Confirmed live, a normal linked worktree of a repository with packed objects returned the linked branch and expected first commit. Its private view reported the expected tracked modification. A native populated-submodule probe used the parent and child readers separately. The parent index contained the child's expected commit, parent status with submodule traversal suppressed was empty, and the child's private view reported its tracked modification. Both probes checked that source names, modes and contents stayed unchanged and that private views were removed. Item 5 can build on this interface. Its child readers should keep each child's roots and use the separate service reader for service-selected defaults. The item 5 acceptance selector printed `testing: warning: no tests to run` at the reviewed commit. The installed-service and integrated checks still need the built candidate, and the macOS test binary cross-compiled without a native run. [Snapshot handoff](../../../../src/client/internal/folders/checkout.go#L1239) · [Reader changes](../../../../.foundry/go-rewrite/results/s4-client-4r-build.md#L58) · [Item 5](../plan.md#L153)

## Closure of the first review

| Prior finding | Outcome |
|---|---|
| post-4 1 | Partly closed. The frozen direct-file cases and the service-default control passed. The unverified Git root remains open under post-4b 3. |
| post-4 2 | Partly closed. The frozen tests refused a shared clone and preexisting links and admitted local packed objects. The scan and lifetime defects are post-4b 1 and post-4b 2. Issue 1684 records the deliberate shared-store refusal. |
| post-4 3 | Closed for valid branch links. Both the loose and packed branch fixtures passed. The broken-link normalization is the new defect in post-4b 4. |
| post-4 4 | Closed for discovery's byte guard. The multibyte boundary passed without a deep directory and rejected a limit of 8,193 bytes. The two full native path fixtures still skipped under issue 1678. |

Issue 1679 still tracks the installed Git process receiving the original configuration during parsing. It was outside this repair. The observed origin-field behavior matches the plan's credential-free output rules. Foundation rule R9 also covers logs and reported data. [First review](post-build-4.md#L9) · [Native boundary skips](../../../../docs/issues.md#L7929) · [Configuration process issue](../../../../docs/issues.md#L7938) · [Shared-store issue](../../../../docs/issues.md#L7980) · [Foundation rule R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370)

## Test commands and mutation results

Confirmed live as ordinary user identifier 1000 on Linux with Git 2.47.3 and Go 1.26.0, the item 4 acceptance command exited zero with 60 named passes and two skips. [Plan command](../plan.md#L151)

```text
$ go test -v -race ./src/client/internal/folders -run 'TestInspectFolderMetadata|TestInspectFolderRefusals|TestInspectFolderOrigin|TestCheckoutSnapshot' -count=1
--- PASS: TestInspectFolderMetadata (0.76s)
--- PASS: TestInspectFolderRefusals (0.11s)
    --- SKIP: TestInspectFolderRefusals/resolved_path_of_8192_bytes (0.03s)
    --- SKIP: TestInspectFolderRefusals/resolved_path_of_8193_bytes (0.03s)
--- PASS: TestInspectFolderOrigin (0.88s)
--- PASS: TestCheckoutSnapshot (0.66s)
PASS
ok      github.com/qnlbnsl/lgtm-studio/src/client/internal/folders  3.459s

$ go test -v -race ./src/client/internal/folders -run 'TestCheckoutSnapshot|TestCheckoutReview' -count=1
--- PASS: TestCheckoutReviewOutsideFiles (0.85s)
--- PASS: TestCheckoutReviewObjectStores (0.23s)
--- PASS: TestCheckoutReviewSymbolicHead (0.35s)
--- PASS: TestCheckoutReviewResolvedPathLimit (0.12s)
--- PASS: TestCheckoutSnapshot (0.51s)
PASS
ok      github.com/qnlbnsl/lgtm-studio/src/client/internal/folders  3.092s
```

Each mutation below was applied alone in the scratch copy. Each exited one on the intended assertion. [Planned mutations](../plan.md#L150)

| Mutation | Observed failure |
|---|---|
| Prefer the checked-out branch over the remote head. | The divergent-branches fixture reported the wrong main branch and first commit. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L193) |
| Use the common index in a linked worktree. | The private view lost the worktree's complete split index. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L537) |
| Enable configuration includes in the parser. | The local origin ceased to be independent of the inert fixture include. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L616) |
| Keep synthetic origin user information. | The result differed from the permitted credential-free address. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L473) |
| Raise the resolved-path limit to 8,193 bytes. | Discovery returned facts for the one-byte-over boundary. [Assertion](../../../../src/client/internal/folders/checkout_review_test.go#L549) |

The full frozen suite also exited zero with a deliberately wrong implementation that refused Git directories stored directly under the parent's `modules` directory. The populated-submodule probe failed on that same mutation with the fixed not-readable refusal. This gap matters for item 5's child inspection gate. [Child inspection requirement](../plan.md#L156)

**Grade 6 out of 10.**
