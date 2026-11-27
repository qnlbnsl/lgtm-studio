# REVISE

The shared text filter can create and emit a complete registered secret when the redaction marker joins the surrounding text.

The test writer, builder and reviewer were OpenAI workers. The reviewer authored neither the build nor the frozen tests. The review is for item 7 on `sprint-0`, using the Link diff from `b637da6e~1` through `624ad929`. The task is `postbuild-go-s4-client-folders-7` with nonce `5a73e4`. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s4-client-folders-7.md) · [Builder handoff](../../../../.foundry/go-rewrite/results/s4-client-7-build.md)

## Post-build checks

The plan and findings register have no item 7 entry marked `UNREVIEWED FIX`. The fold assigned zero findings to post-build checks. For the executable-filter scenario, the Linux Link tests confirmed a fixed branch refusal and a separate origin omission at durable append. Those fixtures invoke the common append boundary directly. The signed-peer history fixtures confirmed that required unsafe display text, exact fields and unknown fields stop both approved and waiting clients with one recovery instruction and unchanged database bytes. The safe control preserved the original stored bytes and sent the same event number and field values. The late-registration control stopped before handing the newly unsafe event to the sender. [Plan checks](../plan.md#L265) · [Findings register](round-1-findings.md#L5) · [Fold](../../../../.foundry/go-rewrite/results/fold-go-s4-client-folders-r1.md#L9) · [Folder append fixture](../../../../src/client/internal/link/filter_test.go#L187) · [Legacy replay fixtures](../../../../src/client/internal/link/filter_test.go#L544)

The history controls also passed for an out-of-range counter before dial, a corrupt required suffix after Welcome and drainage of the last legal event. Direct executable captures, native macOS, the paired server and website checks and installed-supervisor observations are release-wave checks. [History fixtures](../../../../src/client/internal/link/revision_two_recovery_test.go#L27) · [Release handoff](../../../../.foundry/go-rewrite/results/s4-client-7-build.md#L57)

## Findings

### post-7 1 — MAJOR — Redaction can recreate a registered secret

**Statement.** Foundation rule R9 keeps secrets out of logs, the board and the website. Item 7 supplies the shared filter for complete and piecewise display text. [Foundation rule R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370) · [Item 7](../plan.md#L213)

**Evidence.** By reading, `MaskText` and `TextStream.Push` emit the result of replacing spans with `[redacted]`. Only the report-specific `displayText` screens the transformed value. Confirmed live, both public masking methods emitted a complete registered value in the scratch probe. [Complete-text path](../../../../src/client/internal/link/filter.go#L43) · [Stream path](../../../../src/client/internal/link/filter.go#L86) · [Matcher](../../../../src/client/internal/link/filter.go#L130) · [Report safeguard](../../../../src/client/internal/link/filter.go#L215)

**Failure scenario.** Register `fixture-first-secret` and `fixture-[redacted]-value`. Mask `before fixture-fixture-first-secret-value after` once as a complete string and once byte by byte. Both outputs contain the second registered value, which was absent from the input.

**Fix.** Screen the transformed output in both public paths. Keep enough output pending to classify matches that span a replacement and adjacent text, and omit any span for which rendering the marker would still expose a registered value.

### post-7 2 — MAJOR — A scan can rewrite a receipt after its identifier becomes unsafe

**Statement.** Item 7 requires the exact identifier guard to prevent an unsafe identifier from being written after registration or handed to the sender. [Admission and result rules](../plan.md#L215) · [Required assertion](../plan.md#L218)

**Evidence.** Confirmed live with a signed peer, a scan admitted while its identifier was safe rewrote its receipt from started to finished after the filter learned that identifier. The client suppressed the result and logged the fixed invalid-frame diagnostic. By reading, scan settlement calls `store.finish` before the sender check, and the receipt transaction writes the identifier as the key. The frozen queued-result case registers the value after the receipt commit. [Scan settlement](../../../../src/client/internal/link/commands.go#L281) · [Receipt write](../../../../src/client/internal/link/receipts.go#L365) · [Sender check](../../../../src/client/internal/link/session.go#L724) · [Frozen queued-result case](../../../../src/client/internal/link/filter_test.go#L433)

**Failure scenario.** Admit a scan and hold the worker after receipt creation. Register its command identifier as a synthetic secret while the worker is held. Release the worker and reopen the private database after stopping the client. The command has a finished receipt under the newly unsafe identifier.

**Fix.** Recheck the exact identifier under the registration lock through the asynchronous receipt transaction. Preserve the started receipt and take the fixed invalid-frame path when the identifier has become unsafe. Add a signed-peer case that registers the value before worker completion and observes both disk and sender handoff.

### post-7 3 — MINOR — An overlapping match can leave a credential suffix visible

**Statement.** Item 7 requires the matcher to hold a possible suffix across chunks, including overlapping matches. [Text filter contract](../plan.md#L213)

**Evidence.** By reading, after selecting the longest match at a position, the matcher jumps to its end and skips every start inside that match. Confirmed live, the complete and chunked paths both exposed the suffix of a second known value that started inside the first and ended later. The frozen fixture covers overlapping prefixes and a self-overlapping prefix. [Matcher](../../../../src/client/internal/link/filter.go#L102) · [Frozen text cases](../../../../src/client/internal/link/filter_test.go#L27)

**Failure scenario.** Register `alpha-BETA` and `BETA-gamma`. Mask `before alpha-BETA-gamma after` as one string, then split it immediately after `alpha-BETA`. Both outputs are `before [redacted]-gamma after`, leaving the second credential's tail visible.

**Fix.** Extend the masked span through every intersecting match and hold the span while a chunk boundary can still extend it. Add whole-text and split-chunk cases for matches that start and end at different offsets.

## Linux test results

The plan's command in the shared checkout exited 1 when private-state fixtures rejected the sandbox ownership of the filesystem root. The tests ran in one disposable archive of `624ad929`. For that archive only, the state helper treated the sandbox-mapped owner as root for `/` and `/tmp` and still checked permissions. The reviewed Link source and frozen tests were unchanged for the baseline. [Acceptance command](../plan.md#L221) · [Parent ownership check](../../../../src/client/internal/state/folder.go#L166)

```text
$ go test -v -race ./src/client/internal/link -run 'TestOutgoingFilterText|TestOutgoingFilterReports|TestOutgoingFilterLegacyReplay' -count=1
--- PASS: TestOutgoingFilterText (0.00s)
--- PASS: TestOutgoingFilterReports (1.63s)
--- PASS: TestOutgoingFilterLegacyReplay (0.33s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/client/internal/link  3.020s
```

The focused run recorded zero skips. The frame-debug guard, terminal-history suite and handshake fence also printed `PASS`, with Link at `2.700s`. The full native Linux command `go test -v -race ./src/client/...` exited 0 across 11 packages, with Link at `23.951s`. The Link package skipped its root-only cross-owner test. The other three skips were the command subprocess entry and two folder path-limit cases. [Frame-debug test](../../../../src/client/internal/link/conn_frame_log_test.go#L28) · [Client gate](../plan.md#L245)

## Mutation results and a gap in the frozen assertions

Each planned mutation was applied separately in the disposable archive. Each selected frozen test exited 1 on an assertion. [Planned mutations](../plan.md#L220)

| Change | Observed assertion |
|---|---|
| Truncate display text before masking. | `filter_test.go:72` reported incorrect wire machine facts. |
| Mask a resolved path in place. | `filter_test.go:240` reported a different durable folder event. |
| Match each piece independently. | `filter_test.go:49` reported failure to hold a split match or preserve the surrounding text. |
| Register the Welcome token after the first facts append. | `filter_test.go:72` reported incorrect wire machine facts. |
| Bypass filtering at pending flush. | `filter_test.go:86` reported the newly known token in the durable replacement and failure to replay the filtered replacement. |
| Bypass the queued-result check. | `filter_test.go:94` reported an unsafe sender handoff and no fixed invalid-frame diagnostic. |
| Bypass retained-report screening at preflight and handoff. | `filter_test.go:552` reported a handoff before full suffix validation and zero recovery instructions. |

Removing the partial-character hold from `maskPrefix` passed all three frozen item 7 suites and the full Link package. The full mutated Link run printed `PASS` and `ok` at `25.816s`, with the same root-only skip. The frozen stream case checks concatenated output, which can hide an invalid returned piece. A scratch probe that checked `Push` separately confirmed that the mutation emitted the first byte of a three-byte character by itself. The probe passed with the original code. [Character hold](../../../../src/client/internal/link/filter.go#L115) · [Frozen assertion](../../../../src/client/internal/link/filter_test.go#L38)

**Grade.** 7 out of 10.
