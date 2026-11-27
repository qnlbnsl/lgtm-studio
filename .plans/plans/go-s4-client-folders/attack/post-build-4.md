# REVISE

The checkout can direct the client to read files outside the working tree and its Git metadata directories.

Two OpenAI workers from my vendor wrote the frozen tests and built item 4. I reviewed both at `49d11550` on `sprint-0`. The frozen test file matches `86bc363c`. [Test handoff](../../../../.foundry/go-rewrite/results/s4-client-4-tests.md#L1) [Builder handoff](../../../../.foundry/go-rewrite/results/s4-client-4-build.md#L1)

## Findings

### post-4 1 · MAJOR · plan

Checkout configuration can choose a file outside the checkout for the client to open and copy. The effective-file reader accepts absolute paths, expands the home directory and resolves relative parent components without a containment check. The regular-file reader follows symlinks in the parent path and at the leaf, so fixed metadata names can also reach outside the allowed directories. The frozen linked-worktree test confirms that a checkout-selected excludes file in a separate fixture home affects the private view. A checkout can therefore make the client open another readable account file as an excludes or attributes source. Anchor reads to verified directory descriptors, admit linked-worktree metadata through a validated Git relationship, and refuse checkout-selected files outside those roots. Preserve independently selected service defaults through their own boundary. [Plan](../plan.md#L148) [Effective-file reader](../../../../src/client/internal/folders/checkout.go#L1168) [Regular-file reader](../../../../src/client/internal/folders/checkout.go#L432) [External fixture file](../../../../src/client/internal/folders/checkout_test.go#L517) [Ignore assertion](../../../../src/client/internal/folders/checkout_test.go#L558)

### post-4 2 · MAJOR · plan

The private view hands Git the original object store as a live alternate. Git can honor that store's own alternates, so disabling transports and lazy fetching leaves local object lookups able to cross the Git directory boundary. For example, a shared clone can resolve a commit from a separate object store while discovery performs its history walk. Constrain Git to approved read-only object roots or refuse when alternate stores and escaping links cannot be excluded. The missing-object fixture exercises a remote promisor configuration with one object removed. It does not exercise secondary local object stores. [Plan](../plan.md#L144) [Snapshot object access](../../../../src/client/internal/folders/checkout.go#L687) [History walk](../../../../src/client/internal/folders/checkout.go#L877) [Missing-object fixture](../../../../src/client/internal/folders/checkout_test.go#L633)

### post-4 3 · MAJOR · plan

Discovery reports a checkout with a traditional symbolic `HEAD` file as detached. The reader follows the link and copies the branch's commit identifier into the private `HEAD`, losing the symbolic target. In an isolated native fixture, Git identified `refs/heads/topic` and discovery returned `main` with the checked-out branch absent. Preserve the symbolic target when it points at a valid internal branch reference, and add a fixture for that Git representation. [Branch selection requirement](../plan.md#L145) [Source file read](../../../../src/client/internal/folders/checkout.go#L432) [Private head capture](../../../../src/client/internal/folders/checkout.go#L713) [Frozen branch cases](../../../../src/client/internal/folders/checkout_test.go#L207)

### post-4 4 · MAJOR · plan

The frozen suite on this host accepts an implementation that raises the resolved-path limit to 8,193 bytes. Both boundary fixtures skip while building the deep checkout, before either reaches discovery. The isolated byte predicate accepted 8,191 and 8,192 bytes and rejected 8,193 bytes with multibyte input. This verifies the comparison itself. The native acceptance cases remain open under issue 1678. Add a repeatable test that forces both lengths through discovery's validation and keep the native cases for full path resolution. [Path contract in the plan](../plan.md#L143) [Discovery guard](../../../../src/client/internal/folders/checkout.go#L75) [Boundary fixtures](../../../../src/client/internal/folders/checkout_test.go#L348) [Issue 1678](../../../../docs/issues.md#L7929)

### post-4 5 · MINOR · plan

Raw origin credentials cross into a separate trusted Git process before sanitization. The client pipes the complete original configuration into `git config` and reads its listing back into Go. An origin password is therefore present in the parser subprocess even when the returned facts contain only the credential-free address. The output behavior is consistent with foundation rule R9 on logs and reported data. To meet the assignment's stricter function boundary, parse and sanitize the configuration in-process. [Config pipe](../../../../src/client/internal/folders/checkout.go#L614) [Subprocess input and diagnostics](../../../../src/client/internal/folders/checkout.go#L391) [Published field assignment](../../../../src/client/internal/folders/checkout.go#L899) [Foundation rule R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370)

## Verification on Linux

The plan's previously unreviewed mechanism is the private snapshot handed from discovery to status. The linked-worktree and split-index fixtures passed. They checked the private view's head, local commit access, index, settings and ignore sources. The source-preservation assertions compared names, modes and file contents. The two replacement fixtures overlapped capture with atomic replacements of the index or configuration and accepted a coherent view or a cleaned-up refusal. The missing-object and executable-configuration fixtures passed with zero observed requests and no execution marker. The frozen cancellation check starts with a cancelled context. A separate scratch probe confirmed that cancellation after private-directory allocation returned an error and left the directory empty. The populated-child and installed-service observations require item 5 and the integrated candidate. Native macOS execution remains open. [Review register](round-1-findings.md#L38) [Snapshot mechanism](../plan.md#L147) [Native post-build check](../plan.md#L269) [Snapshot fixtures](../../../../src/client/internal/folders/checkout_test.go#L479) [Replacement observer](../../../../src/client/internal/folders/checkout_test.go#L933)

By reading, item 4 executes installed Git with fixed plumbing, clean configuration inputs, disabled transport and discarded diagnostics. The writes in the reviewed operations target the disposable view. The origin fixtures confirmed field omission and user information removal, and the returned error paths use fixed refusals. The raw subprocess boundary and external reads are recorded above. [Executable selection](../../../../src/client/internal/folders/checkout.go#L287) [Git subprocess policy](../../../../src/client/internal/folders/checkout.go#L378) [Private directory and writes](../../../../src/client/internal/folders/checkout.go#L504) [Snapshot writer](../../../../src/client/internal/folders/checkout.go#L561) [Origin fixtures](../../../../src/client/internal/folders/checkout_test.go#L428)

The planned race-enabled command ran as user identifier 1000 on Linux with Git 2.47.3 and Go 1.26.0. It exited zero with 60 named passes and two skips. A diagnostic run in the scratch copy located both skips in `mkdirat` while copying the initialized checkout into the deep fixture. [Acceptance command](../plan.md#L151) [Fixture copy](../../../../src/client/internal/folders/checkout_test.go#L393)

```text
--- PASS: TestInspectFolderMetadata (0.74s)
--- PASS: TestInspectFolderRefusals (0.11s)
    --- SKIP: TestInspectFolderRefusals/resolved_path_of_8192_bytes (0.03s)
    --- SKIP: TestInspectFolderRefusals/resolved_path_of_8193_bytes (0.03s)
--- PASS: TestInspectFolderOrigin (0.86s)
--- PASS: TestCheckoutSnapshot (0.69s)
PASS
ok      github.com/qnlbnsl/lgtm-studio/src/client/internal/folders  3.449s
```

I ran each planned mutation separately in one scratch copy against the corresponding frozen test. Each exited one on its intended assertion. [Planned mutations](../plan.md#L150)

| Mutation | Failing assertion |
| --- | --- |
| Prefer the checked-out branch. | The selected main branch and first commit were wrong. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L193) |
| Use the common index in a linked worktree. | The private view lost the linked worktree's complete split index. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L540) |
| Enable configuration includes. | Discovery lost the local origin independent of the included configuration. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L619) |
| Return origin user information. | The result differed from the permitted credential-free address. [Assertion](../../../../src/client/internal/folders/checkout_test.go#L473) |

**Grade 5 out of 10.**
