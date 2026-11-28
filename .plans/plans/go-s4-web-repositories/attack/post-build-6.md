# REVISE

A late save can close the Add repository dialog after its path has changed, and the same race can show inspection facts for the previous path.

The builder and the writer of the frozen tests were OpenAI workers, as is this reviewer. I did neither the build nor the test writing. This review covers `fe8f4f3d..a2b09fd1` on `sprint-0`, for task `postbuild-go-s4-web-repositories-6` and nonce `ed19bc`. Source line references use the reviewed revision. The Add repository test file was already frozen at the base and is byte-identical at both ends.

## Findings

### post-6 1. MAJOR. An obsolete reply can affect the current input

**Statement.** Plan item 6 requires an edit to clear old inspection facts, cancel obsolete attempts and end the save intent. The sprint design uses inspection facts only for the machine and path that were asked. [Plan item 6](../plan.md#L184) · [Sprint design](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L272)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. I released a decoded reply and dispatched a path edit on the promise queue after the wrapper's last validity check and before the dialog's continuation. The inspection case left the new path in the input with the old branch in Base branch. In the save case, the watch already contained the original saved identity. The old save closed the dialog after the edit. The wrapper checks ownership before resolving its promise. The dialog's two success paths consume the value after their own awaits without checking the same attempt again. The edit clears the old attempt, but the old save can then replace the progress state with a watch join. The authoritative watched record from that earlier write remains valid. [Wrapper's final check](../../../../src/frontend/src/server/folders.ts#L66) · [Edit cancellation](../../../../src/frontend/src/screens/service.tsx#L569) · [Inspection continuation](../../../../src/frontend/src/screens/service.tsx#L619) · [Save continuation](../../../../src/frontend/src/screens/service.tsx#L714)

**Fix.** Recheck the specific attempt, dialog and session immediately after each successful await, before updating facts, feedback or save progress. Add a probe that edits between the wrapper and its caller and checks that the new form stays open with its own facts.

### post-6 2. MAJOR. A silent connection can leave Add disabled indefinitely

**Statement.** Plan item 6 gives a lost add reply a replay path within 23 hours. It gives an inconclusive inspection one retry. The contract bounds the time the server waits for a folder operation to 25 seconds. [Plan item 6](../plan.md#L187) · [Folder contract](../../../../src/contracts/proto/lgtm/v1/projects.proto#L13)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. I kept the watch open and let the folder transport accept the request without returning a response or an error. The save still showed “Adding…” with a disabled button after advancing the browser clock past 23 hours. In a separate probe, inspection still showed “Looking at the folder…” after one hour. The folder calls carry cancellation signals but have no client deadline. The website's Connect transport has no default deadline. A connection that stays open without delivering the result can bypass the failure handler, so the save never enters replay and inspection never reaches its automatic retry. [Folder call options](../../../../src/frontend/src/server/folders.ts#L66) · [Transport setup](../../../../src/frontend/src/server/api.ts#L42) · [Save states and button](../../../../src/frontend/src/screens/service.tsx#L711)

**Fix.** Set a finite deadline per attempt, with enough time for the server's documented wait and transport delay. Treat the client deadline as inconclusive. Keep the same save key for an eligible retry and keep the six-second watch join measured from a successful reply. Test a request that supplies neither an answer nor a failure.

### post-6 3. MINOR. The frozen tests allow arbitrary refusal text in the footer

**Statement.** The plan allows the fixed folder refusals and the existing generic messages. Foundation rule 8 specifies an error code with a display sentence, and rule 9 keeps secrets off the website. [Plan copy table](../plan.md#L227) · [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L358)

**Evidence and failure scenario.** The reviewed code uses the fixed allowlist. I changed only its generic refusal fallback in scratch to display the raw error message. The frozen plan selector still passed all 100 tests. An independent browser check with the harmless unrecognized text “Unexpected diagnostic for review fixture.” expected the generic refusal and failed under that mutation. The current code passed that probe at both widths. The test table covers approved refusals and the synthetic missing-folder branch. A regression could expose a backend diagnostic or an unapproved sentence. [Fallback](../../../../src/frontend/src/server/folders.ts#L35) · [Frozen refusal table](../../../../src/frontend/e2e/add-repository.spec.ts#L791) · [Synthetic generic refusal case](../../../../src/frontend/e2e/add-repository.spec.ts#L672)

**Fix.** Send an unrecognized message under a recognized refusal code in the browser fixture. Assert the exact generic footer and the absence of the unrecognized message.

**Grade.** 7 out of 10.

## Known saves, removal and session boundaries

The final plan marks the known-save recheck as an **UNREVIEWED MECHANISM**. The latest pre-build register records it under critic-r1 4 and logic-r1 03. Confirmed live at both widths, the frozen scenario retained matching rows from a lost watch, failed one replacement and kept another silent. The dialog ignored retained rows, refreshed only the repository watch at two seconds and reopened the button at six seconds. A manual recheck had its own six-second bound. A further recheck closed the dialog and restored the previous form after the watch recovered. The case observed one save request, one watched identity and one return. [Plan mechanism](../plan.md#L189) · [Review register](round-1-findings.md#L24) · [Frozen scenario](../../../../src/frontend/e2e/add-repository.spec.ts#L616)

Confirmed live, the matching reply and watch completed in either order. The opposite removal orders preserved the path, selected the remaining machine and stopped the old save from returning to the removed machine. The expiry cases aborted outstanding work and started a clean dialog after sign-in. The replacement dialog discarded a queued authentication refusal. In a separate server-switch probe, a queued successful save from the previous session arrived after the new session opened Add repository with the same watched identity. The new form remained open and untouched and there was no success toast. The selected session and watch suites also covered same-host cookie ordering and late answers from a different host. [Watch order](../../../../src/frontend/e2e/add-repository.spec.ts#L485) · [Removal and expiry](../../../../src/frontend/e2e/add-repository.spec.ts#L694) · [Session tests](../../../../src/frontend/e2e/repository-session.spec.ts)

## Compatibility, visible copy and deployed checks

Confirmed live, before-and-after captures matched in the in-page sample, standalone mock and older-server fixture, at desktop and phone widths. I compared text, button state, selected options, path value, placeholder and focus across five form states in each context. The sample dialog body also matches by reading the pinned diff. The broader discovery cases kept sample work on the compatibility paths, loaded watched records in the advertising server, and discarded pending sample activity on a switch. [Compatibility implementation](../../../../src/frontend/src/screens/service.tsx#L775) · [Discovery scenarios](../../../../src/frontend/e2e/repository-discovery.spec.ts#L188)

By reading the visible branches, the previously existing folder instruction is reused and there is no new instruction sentence. The dirty status, blank machine choice and refusal text follow the recorded approvals. The folder refusal table matches all seventeen fixed contract sentences and their codes. The plan's “The path is not a Git repository.” differs from the contract's “The folder is not a Git repository.” The builder documents that choice and the website shows the contract sentence. [Visible text](../../../../.foundry/go-rewrite/results/s4-web-6-build.md#visible-text-and-its-authority) · [Approval](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L51) · [Contract table](../../../../src/contracts/proto/lgtm/v1/projects.proto#L49)

Deployment checks R12, R13 and R14 remain unverified on an integrated stack. The reviewed server composition does not mount the project service, and the reviewed sidebar still opens Track repository. The browser fixture exercised removal, compatibility and a Machines save followed by reload. Real-machine facts before the first harness scan, two deployed entry points and watched identity pairs from real saves remain for the later integration gate. The paired recovery check belongs to the later recovery item. [Server composition](../../../../src/server/internal/app/start.go#L266) · [Sidebar](../../../../src/frontend/src/app.tsx#L267) · [Post-build scenarios](../plan.md#L281)

## Browser runs and mutation results

I used Node.js `v26.13.0` and pnpm `12.16.1` with the requested Node.js executable on the path. Chromium ran at 1440 by 900 and 390 by 844 pixels. All experiments used one disposable source copy. The first broad run printed `208 passed` and two standalone-mock startup failures. pnpm rejected a linked task-state directory in the scratch environment. A local task-state directory resolved that startup failure, and the two cases passed on rerun.

| Check | Result |
|---|---|
| `pnpm exec playwright test --config src/frontend/playwright.config.ts folder-calls add-repository repository-entry` | `100 passed (44.3s)`, exit 0. The pinned revision contains the item 5 and 6 specs. Item 7's `repository-entry` spec is absent at this revision. |
| Run `add-repository repository-discovery repository-session repository-watch folder-calls`. | `208 passed (1.9m)` and two scratch setup failures, exit 1. Both affected discovery cases passed in the eight-case compatibility rerun. |
| Compare the six base captures against the six reviewed-revision captures. | The base run printed `6 passed (7.7s)`. The reviewed-revision run with the standalone discovery reruns printed `8 passed (13.6s)`. The capture diff was empty. |
| Probe the continuation race, a silent save and an unknown refusal. | `6 failed`, exit 1. Both races and the silent save failed at both widths. The two generic refusal cases passed. |
| Probe a silent inspection and a decoded save across a server switch. | `2 failed`, exit 1. The inspection remained pending at both widths. Both switch cases passed. |
| Allow late inspections to apply after edits. | `2 failed`, exit 1. An obsolete branch and an obsolete refusal reached the form. [Assertions](../../../../src/frontend/e2e/add-repository.spec.ts#L385) |
| Join on any folder for the submitted machine. | `1 failed`, exit 1. The other folder closed the form. [Assertion](../../../../src/frontend/e2e/add-repository.spec.ts#L510) |
| Accept retained records when the repository watch is down. | `1 failed`, exit 1. The retained row closed the form. [Assertion](../../../../src/frontend/e2e/add-repository.spec.ts#L628) |
| Extend a join past six seconds. | `1 failed`, exit 1. Network feedback was missing after the deadline. [Assertion](../../../../src/frontend/e2e/add-repository.spec.ts#L649) |
| Allow replay after 23 hours. | `2 failed`, exit 1. Both clock tests saw another save where they expected inspection. [Assertion](../../../../src/frontend/e2e/add-repository.spec.ts#L603) |
| Insert the reply's folder into the watched records. | `1 failed`, exit 1. The repository changed before the matching watch frame. [Assertion](../../../../src/frontend/e2e/add-repository.spec.ts#L509) |
| Show the raw unrecognized refusal message. | `100 passed (46.0s)`, exit 0, in the frozen plan selector. The independent exact-footer probe printed `1 failed`, exit 1. |
| `git diff --check fe8f4f3d..a2b09fd1 -- src/frontend` | Exit 0 with no output. |

Suggested first project journal entry for approval. On November 27, the Add repository review matched 30 sample dialog states and found that a late reply could close a form after its path changed, even though all 100 planned browser tests passed.
