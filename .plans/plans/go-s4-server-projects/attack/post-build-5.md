# SHIP

The attachment code takes the required repository lock, but the frozen tests pass if a new folder skips that lock, which would turn a concurrent removal into a database error.

OpenAI supplied the test writers, the builders and this reviewer. I wrote neither the shipped code nor the frozen tests. I reviewed Projects item 5 and the three item 4 repairs on `sprint-0` at commit `c64a0170`, using `a3ce355b~1..c64a0170` for the Projects diff. The task identifier is `postbuild-go-s4-server-projects-5` and the nonce is `196d1a`.

## Findings

### post-5 1 · MINOR · The target-lock test covers a refusal and leaves new attachments unguarded by the suite

**Statement.** Plan item 5 requires the attachment to lock the explicit target before deciding ownership or changing membership. Plan item 6 uses the same row lock before the orphan decision. [Attachment lock](../plan.md#L188) · [Removal lock and race test](../plan.md#L202)

**Evidence.** By reading, the production query takes `FOR UPDATE` for every explicit target. The frozen test holds a conflicting target lock only when the path already belongs to another repository. Confirmed live, changing the query to lock only when the path already exists passed all 11 frozen attachment, report, settlement and comparison groups in `36.896s`. [Production query](../../../../src/server/internal/projects/reports.go#L172) · [Frozen lock case](../../../../src/server/internal/projects/repositories_test.go#L224) · [Blocking assertion](../../../../src/server/internal/projects/repositories_helpers_test.go#L86)

**Failure scenario.** A new folder reports a matching origin while the target's last folder is being removed. Confirmed live against the mutation, a modeled removal held the target lock with the old row still visible. The report read the old target, then failed the folder insert after removal committed. Link returned a database error in place of a committed `repository_not_found` refusal. The pinned production code passed the same probe.

**Fix.** Keep the planned `TestProjectAttachAgainstRemoval` on the integration gate and require it to fail if a new attachment skips the target lock. In its removal-first case, assert that a matching new path waits for the locked target, then advances the Link mark with the fixed refusal and emits no attachment save. [Required race coverage](../plan.md#L205)

## The unreviewed feed change and the attachment race

The round-one register's logic-r1 02 and its three duplicates flag the replacement for a removal that exceeds Store's change limit. Item 5 supplies the ordinary repository save and the reserved `refresh` key. By reading, attachment records the generated version 7 universally unique identifier (UUID) with empty scope, so an ordinary attachment cannot produce the refresh key. Store adds the feed reference in the same write. Confirmed live, two aliases produced one saved reference. Each later insertion produced one more. Reuse, refusals and the interrupted write produced zero. [Register](round-1-findings.md#L10) · [Item 5 feed requirement](../plan.md#L191) · [Reserved key](../../../../src/server/internal/projects/repositories.go#L11) · [Attachment feed write](../../../../src/server/internal/projects/reports.go#L194) · [Store feed transaction](../../../../src/server/internal/store/feed.go#L120) · [Feed assertions](../../../../src/server/internal/projects/repositories_helpers_test.go#L226)

Confirmed live, a scratch PostgreSQL probe used the real Link report handler and modeled item 6's machine lock, repository lock, folder deletion and orphan decision in a separate Store write. Each lock ordering used a fresh database. [Post-build scenarios](../plan.md#L279) · [Planned removal write](../plan.md#L199)

| Lock order | Committed observation |
|---|---|
| Attachment first. | The remover waited for the attachment. One repository and the new machine's folder survived. The stored Add reply contained both folders as of the earlier write, and a same-key retry returned those exact bytes after removal. The report mark was 1. The repository saves landed at feed positions 11 and 13. |
| Removal first. | The attachment waited for the remover. Zero repository and folder rows survived. The report mark was 1 and the operation stored `repository_not_found` with no reply bytes. Position 11 was the sole repository reference, a removal. |

The pinned app has no Projects wiring, the Projects watch is a test stub, and the paired test listing lacks `TestSprintFourFolderSettlement`. The modeled race verifies the attachment side of the lock contract. The real removal callback, the large-removal refresh and watch reopen, and the paired client and browser observations belong to the later gates. [App wiring](../../../../src/server/internal/app/start.go#L266) · [Watch stub](../../../../src/server/internal/projects/watch.go#L11) · [Assembled refresh case](../plan.md#L252) · [Paired case](../plan.md#L275)

## Attachment behavior against the plan

- By reading, Link takes the machine lock before invoking Projects. Projects selects only the open operation for the authenticated machine and matching operation and command identifiers. The same transaction looks up the exact resolved path, stores membership and the complete Add reply, and advances the Link mark before acknowledgement. Confirmed live, the signed fixture held one alias report while the second waited, then returned the same identity to both callers. A later reuse with a changed origin still returned the saved folder. Distinct paths on one machine and the same path on a different machine received distinct folder identifiers. [Machine lock and event order](../../../../src/server/internal/link/inbox.go#L35) · [Acknowledgement](../../../../src/server/internal/link/stream.go#L144) · [Operation and reply write](../../../../src/server/internal/projects/reports.go#L49) · [Alias test](../../../../src/server/internal/projects/repositories_test.go#L44)
- By reading, two present origins compare by their normalized forms. First-commit equality is available only when at least one origin is known absent. An existing target keeps its original metadata. Missing explicit targets use the same fixed refusal for saved and unsaved paths. The projection uses one join ordered by folder identifier and checks the row iterator error. Confirmed live, the attachment tests passed the identity and precedence cases and observed unchanged membership and zero saves for refusals. [Matching and projection](../../../../src/server/internal/projects/repositories.go#L23) · [Ownership precedence](../../../../src/server/internal/projects/reports.go#L172) · [Identity tests](../../../../src/server/internal/projects/repositories_test.go#L145)
- Confirmed live, interruption after membership and reply creation left the operation open with its original bytes, preserved the staged outcome, left the mark at zero, and committed neither membership nor a save. No acknowledgement was sent. Reopening the same database and retrying produced one save and one terminal reply. Duplicate delivery preserved the rows, and another restart recovered the original reply while offline. The signed credential-bearing-origin case left the operation and feed untouched and kept the synthetic credential out of the Link log. The real client Folders component separately passed its physical alias and origin filtering cases. [Rollback and offline replay](../../../../src/server/internal/projects/repositories_test.go#L286) · [Origin filtering test](../../../../src/server/internal/projects/repositories_test.go#L266) · [Client aliases](../../../../src/client/internal/folders/checkout_test.go#L162) · [Client origins](../../../../src/client/internal/folders/checkout_test.go#L428)

The evidence covers foundation decision D5 and rules R3, R4, R7, R8, R9 and R10 for this item. Projects uses its own tables, Store controls the write and feed, the result survives a reopen, refusals map to fixed sentences, and the fixture uses real PostgreSQL. Issue 1704 is already recorded and concerns the earlier operation wait. [Foundation decisions](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L22) · [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L358) · [Issue 1704](../../../../docs/issues.md#L8151)

## Item 4 repair dispositions

| Prior finding | Disposition and evidence |
|---|---|
| post-4 1 | Closed. By reading, Projects checks lexical equality with `path.Clean` before reading the operation or attempting attachment. Confirmed live, signed Inspect and Add reports with doubled separators, dot components, parent components or trailing separators were acknowledged without rewriting the operation or saving membership. A later clean Inspect report settled the operation. Removing the guard failed both signed test groups. [Validation](../../../../src/server/internal/projects/reports.go#L127) · [Signed cases](../../../../src/server/internal/projects/reports_review_test.go#L15) |
| post-4 2 | Closed. By reading, comparison formats a valid nondefault port from its numeric value. Confirmed live, both spellings of port 2222 produced the same key, and Inspect preserved the original leading-zero spelling. Reinstating the original port digits failed the direct decimal test. [Port comparison](../../../../src/server/internal/projects/origins.go#L72) · [Direct case](../../../../src/server/internal/projects/origins_review_test.go#L5) · [Inspect case](../../../../src/server/internal/projects/reports_review_test.go#L69) |
| post-4 3 | Closed. Confirmed live, returning the raw origin after validation failed the direct comparison test on case folding, the default port and absolute Secure Shell (SSH) shorthand. The unmodified suite passed the equivalent and distinct cases. [Comparison assertions](../../../../src/server/internal/projects/origins_review_test.go#L15) |

## Verification at the pinned commit

I ran the two plan commands in one disposable copy with a writable Go cache. Their selectors excluded the intentionally red watch and router groups. The long-running Link package and the listed intermittent client groups were outside those runs. The item 5 gate selected three attachment groups with zero skips. The three item 6 groups are absent from the pinned commit. The Store and Projects gate passed 35 top-level groups with zero skips. [Item 5 gate](../plan.md#L215) · [Item 4 gate](../plan.md#L165) · [Shared test rules](../../../../.foundry/go-rewrite/common-build.md#L41)

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/projects \
  -run '^TestProject(Attachment|MachineRemoval|ConcurrentOrphanRemoval|AttachAgainstRemoval)' -count=1
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/store ./src/server/internal/projects \
  -run '^Test(HeldReceipt|Migrate|Prune|WriteOnce|RecordValidationAndLimit|Project(Migration|Storage|Operation|Receipt|Replay|Report|FolderSettlement))' -count=1
```

```text
--- PASS: TestProjectAttachmentAliases (3.24s)
--- PASS: TestProjectAttachmentIdentity (8.71s)
--- PASS: TestProjectAttachmentRollback (0.84s)
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  12.808s
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/store     83.422s
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  81.080s
```

The focused race run passed all eight attachment and repair groups in `22.686s`. The two client Folders component groups passed in `1.311s`. Projects `go vet` and the scoped diff whitespace check exited zero. The four frozen test files matched their recorded SHA-256 (Secure Hash Algorithm, 256-bit) digests. Substituting the test-writer stubs from `96ac4e65` failed all three attachment groups on assertions in `3.958s`. [Item 5 test-writer result](../../../../.foundry/go-rewrite/results/s4-projects-5-tests.md)

Each plan mutation used a separate Go overlay in the same scratch copy and failed on the intended assertion. [Planned mutations](../plan.md#L197)

| Mutation | Confirmed result |
|---|---|
| Disable the exact-path lookup. | Exit 1. The alias replies had different identities at `repositories_test.go:85`. |
| Let equal first commits override distinct present origins. | Exit 1. `DistinctPresentOrigins` failed to store `repository_mismatch` at `repositories_test.go:203`. |
| Count withholding as known absence. | Exit 1. `SavedOriginWithheld` and `TwoWithheldEqualCommits` failed to store `repository_mismatch` at `repositories_test.go:203`. |
| Check ownership before target existence. | Exit 1. `MissingTargetForSavedPath` failed to store `repository_not_found` at `repositories_test.go:240`. |

**Grade.** 9 out of 10.
