# REVISE

Git can read a tracked submodule’s original metadata during the parent’s index refresh, before the client checks the child.

The test writer, builder, and reviewer were all OpenAI workers. I neither wrote the frozen tests nor built the code. This review covers item 5 and issue 1686 on `sprint-0`, using `8584aec0~1..d5d1bb9f`. Under the owner’s standing decision, the findings do not block the build. [Worker assignments](../../../../.foundry/go-rewrite/run-log.md#L559)

## Findings

### post-5 1 — MAJOR — Parent status can read original submodule metadata

**Statement.** Item 5 of the plan requires the parent to suppress implicit child inspection and inspect each populated child through its own private view. [Plan](../plan.md#L156)

**Evidence by reading.** The client runs `git status` against the original working tree with `--ignore-submodules=all`, then runs its explicit child checks. In Git 2.47.3, `status` refreshes the index without the flag that skips submodule entries. The refresh can resolve a submodule’s current reference using its original repository. The frozen child test observes execution of a configured program, so it can pass when the refresh merely reads metadata. [Parent status](../../../../src/client/internal/folders/status.go#L84), [Child checks](../../../../src/client/internal/folders/status.go#L24), [Frozen assertion](../../../../src/client/internal/folders/status_test.go#L342), [Git status](https://github.com/git/git/blob/v2.47.3/builtin/commit.c#L1476), [Git index refresh](https://github.com/git/git/blob/v2.47.3/read-cache.c#L1453), [Git submodule comparison](https://github.com/git/git/blob/v2.47.3/read-cache.c#L248), [Git submodule reference store](https://github.com/git/git/blob/v2.47.3/refs.c#L1913)

**Failure scenario.** A populated child has a metadata marker that the explicit child check would refuse. Git can already have read metadata through that marker before the client returns the refusal. This is an additional access path before the admission boundary discussed in issue 1685. [Child admission](../../../../src/client/internal/folders/status.go#L403), [Issue 1685](../../../../docs/issues.md#L7987)

**Fix.** Keep the cached diff for staged submodule changes. Use a private comparison view that excludes submodule entries for the unstaged pass, and leave child reads to the explicit inspection path. Verify that the parent pass performs zero reads of original child metadata.

### post-5 2 — MAJOR — The filter check can read attributes outside the checkout

**Statement.** Item 5 of the plan requires isolated status and a fixed refusal when status cannot be established safely. [Plan](../plan.md#L153)

**Evidence by reading.** The filter preflight sends tracked names from the index to Git’s attribute lookup against the live working tree. The path validation is lexical. Git reads per-directory attribute files using an open that prevents following a link at the final component. Parent directory components can still redirect the read. [Path validation](../../../../src/client/internal/folders/status.go#L127), [Filter preflight](../../../../src/client/internal/folders/status.go#L169), [Git attribute lookup](https://github.com/git/git/blob/v2.47.3/attr.c#L982), [Git attribute file reader](https://github.com/git/git/blob/v2.47.3/attr.c#L685), [Git file open](https://github.com/git/git/blob/v2.47.3/wrapper.c#L694)

**Failure scenario.** A directory containing a tracked path resolves outside the checkout. The preflight can read attributes from that location before deciding to refuse or report a dirty checkout.

**Fix.** Gather the effective working-tree attribute files through descriptor-relative, contained reads and give Git private copies. Use the fixed refusal when a required attribute source cannot be captured safely.

### post-5 3 — MAJOR — Tracked-file reads can leave the checkout during a concurrent change

**Statement.** Item 5 of the plan requires the tracked-file comparison to use the isolated status view. [Plan](../plan.md#L155)

**Evidence by reading.** The private view contains the index and configuration. For working-tree commands, the client passes the live root to Git by path. Git’s index refresh checks path components and file type before a separate pathname open to compare regular-file contents. These operations do not bind the later content read to the entry that was checked. The final client check covers recorded configuration sources. [Status invocation](../../../../src/client/internal/folders/status.go#L84), [Working-tree handoff](../../../../src/client/internal/folders/status.go#L103), [Final source check](../../../../src/client/internal/folders/checkout.go#L766), [Git refresh](https://github.com/git/git/blob/v2.47.3/read-cache.c#L1308), [Git file comparison](https://github.com/git/git/blob/v2.47.3/read-cache.c#L213)

**Failure scenario.** A tracked path changes while Git is checking it. Git can open a file beyond the checkout’s boundary during the subsequent content comparison. This concerns working-tree contents, which item 5 newly reads. Issue 1685 records the corresponding risk for the object store. [Issue 1685](../../../../docs/issues.md#L7993)

**Fix.** Enforce the read boundary at open time. Run working-tree commands against a private view populated through bounded, descriptor-relative reads, or use an operating-system boundary that confines Git’s reads throughout the command.

## Native inspection and issue 1686

The round-one register records the private capture and status split under `logic-r1 09` and `threat-r1 08`. The plan labels it `UNREVIEWED MECHANISM` and assigns the native read-only scenario to it. [Register](round-1-findings.md#L38), [Plan](../plan.md#L147), [Native post-build check](../plan.md#L269)

Confirmed live on Linux with Git 2.47.3 and Go 1.26.0 as an unprivileged user. The linked-worktree and split-index fixture distinguished clean and dirty states. The populated-child fixtures distinguished a clean child, a changed target, and a dirty child. The missing-object fixture produced the fixed refusal with no transport marker. The fixtures checked source contents and modes and verified private-view cleanup after success, refusal, and cancellation. The source comparisons provide evidence about changes to the checkout. The program and transport markers provide evidence about execution and transport. The three read paths above were assessed by reading the code. [Native fixtures](../../../../src/client/internal/folders/status_test.go#L220), [Refusal and cancellation fixtures](../../../../src/client/internal/folders/status_test.go#L377)

The item 5 acceptance command exited zero. All 29 subtests passed with zero skips.

```text
$ go test -v -race ./src/client/internal/folders -run 'TestInspectFolderFacts|TestInspectFolderReadOnly|TestInspectFolderNoSideEffects' -count=1
--- PASS: TestInspectFolderFacts (1.72s)
--- PASS: TestInspectFolderReadOnly (1.66s)
--- PASS: TestInspectFolderNoSideEffects (0.95s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/client/internal/folders  5.369s
```

The separate capture check passed, including sparse selection, missing objects, cancellation, and concurrent replacement of the index and configuration.

```text
$ go test -v -race ./src/client/internal/folders -run '^TestCheckoutSnapshot$' -count=1
--- PASS: TestCheckoutSnapshot (0.67s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/client/internal/folders  1.710s
```

Issue 1686 is confirmed live for the reported whitespace-suffixed symbolic `HEAD`. Discovery returned the fixed refusal with no branch facts. The existing loose and packed branch controls and the tag control also passed. By reading, the repair examines the symbolic link target as text and leaves the link unopened. [Repair](../../../../src/client/internal/folders/checkout.go#L724), [Regression](../../../../src/client/internal/folders/status_test.go#L442)

```text
$ go test -v -race ./src/client/internal/folders -run '^TestInspectFolderRefusesWhitespaceInLinkedHead$|^TestCheckoutReviewSymbolicHead$' -count=1
--- PASS: TestCheckoutReviewSymbolicHead (0.40s)
--- PASS: TestInspectFolderRefusesWhitespaceInLinkedHead (0.07s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/client/internal/folders  1.521s
```

Native macOS acceptance remains open. The real-link checks R1 through R4 and R7 belong to the integrated candidate. The production Link call to `folders.Inspect` was absent from the tree reviewed here. [Acceptance and integration sequence](../plan.md#L160)

## What the frozen tests detect

I ran two of the plan’s mutations in one disposable copy. Leaving assume-unchanged entries suppressed failed at `status_test.go:67`. Bypassing untracked-file detection failed at `status_test.go:109`. Both printed `got clean true, want false` and exited one. The original-configuration and implicit-child-traversal mutations have builder-reported results only. [Frozen tests](../../../../src/client/internal/folders/status_test.go#L97), [Builder’s mutation results](../../../../.foundry/go-rewrite/results/s4-client-5-build.md#L49)

The complete Folders package passed against a deliberately wrong change that returns clean when an unrecognized nested Git marker is the only candidate evidence. The implementation and builder handoff call for a fixed refusal in this case. The change performed the same reads and altered only the result, so it demonstrates a test gap in the safer untracked scan. [Marker handling](../../../../src/client/internal/folders/status.go#L285), [Builder’s intended result](../../../../.foundry/go-rewrite/results/s4-client-5-build.md#L36)

```text
$ go test -race ./src/client/internal/folders -count=1
ok  github.com/qnlbnsl/lgtm-studio/src/client/internal/folders  7.039s
```

**Grade.** 6 out of 10.
